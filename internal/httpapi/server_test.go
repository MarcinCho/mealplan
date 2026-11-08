package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/config"
	"mealplan/internal/domain"
	"mealplan/internal/httpapi"
	"mealplan/internal/nutrition"
	"mealplan/internal/pantry"
	"mealplan/internal/plan"
	"mealplan/internal/product"
	"mealplan/internal/recipe"
	"mealplan/internal/shopping"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

var cheap = auth.Hasher{Time: 1, Memory: 8 * 1024, Threads: 1}

const pw = "correct-horse-battery"

type env struct {
	srv   *httpapi.Server
	st    *store.Store
	svc   *auth.Service
	clock *fakeClock
	imp   *fakeImporter
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st := store.New(storetest.Open(t))
	clock := &fakeClock{t: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)}
	svc, err := auth.NewService(st, auth.Config{SessionIdle: time.Hour, SessionMax: 24 * time.Hour, InviteTTL: 72 * time.Hour, Hasher: cheap}, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Mode: config.ModeSelfhost, CookieSecure: false}
	recipes := recipe.New(st, clock.Now)
	imp := &fakeImporter{}
	products := product.New(st, clock.Now)
	planSvc := plan.New(st, st)
	pantrySvc := pantry.New(st, st, products, clock.Now)
	planSvc.SetDoneHook(pantrySvc)
	deps := httpapi.Deps{Recipes: recipes, Products: products, Plan: planSvc, Nutrition: nutrition.New(st, products, clock.Now), Pantry: pantrySvc, Shopping: shopping.New(st, st, pantrySvc, products), Importer: imp}
	srv := httpapi.New(svc, deps, cfg, http.NotFoundHandler(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &env{srv: srv, st: st, svc: svc, clock: clock, imp: imp}
}

// secondHouseholdClient creates an independent household with its own admin and logs in.
func (e *env) secondHouseholdClient(t *testing.T) *client {
	t.Helper()
	hash, err := cheap.Hash("another-long-password")
	if err != nil {
		t.Fatal(err)
	}
	err = e.st.CreateHouseholdWithAdmin(context.Background(),
		domain.Household{ID: "hB", Name: "Drugi", CreatedAt: e.clock.Now()},
		domain.User{ID: "uB", Login: "bob", PasswordHash: hash, DisplayName: "Bob", CreatedAt: e.clock.Now()}, false)
	if err != nil {
		t.Fatal(err)
	}
	c := e.client(t)
	if rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": "bob", "password": "another-long-password"}); rec.Code != 204 {
		t.Fatalf("login second: %d %s", rec.Code, rec.Body)
	}
	c.useCSRF()
	return c
}

type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]string
	csrf    string
}

func (e *env) client(t *testing.T) *client {
	return &client{t: t, h: e.srv.Handler(), cookies: map[string]string{}}
}

func (c *client) do(method, path string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var rdr io.Reader
	if s, ok := body.(string); ok {
		rdr = strings.NewReader(s)
	} else if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 || ck.Value == "" {
			delete(c.cookies, ck.Name)
		} else {
			c.cookies[ck.Name] = ck.Value
		}
	}
	return rec
}

func (c *client) useCSRF() { c.csrf = c.cookies["mealplan_csrf"] }

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct{ Code string }
	decode(t, rec, &e)
	return e.Code
}

func setup(t *testing.T, c *client) {
	t.Helper()
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "Dom", "login": "marc", "password": pw, "displayName": "Marc"})
	if rec.Code != 204 {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body)
	}
	c.useCSRF()
}

func TestStatusAndSetup(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	var st struct {
		SetupRequired bool
		Mode          string
	}
	decode(t, c.do("GET", "/api/v1/auth/status", nil), &st)
	if !st.SetupRequired || st.Mode != "selfhost" {
		t.Fatalf("status: %+v", st)
	}
	setup(t, c)
	decode(t, c.do("GET", "/api/v1/auth/status", nil), &st)
	if st.SetupRequired {
		t.Fatal("setup must be done")
	}
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "X", "login": "eve", "password": pw, "displayName": "Eve"})
	if rec.Code != 409 || errCode(t, rec) != "setup_done" {
		t.Fatalf("second setup: %d %s", rec.Code, rec.Body)
	}
}

func TestMeRequiresSessionAndReturnsProfile(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	if rec := c.do("GET", "/api/v1/auth/me", nil); rec.Code != 401 || errCode(t, rec) != "unauthenticated" {
		t.Fatalf("anonymous me: %d %s", rec.Code, rec.Body)
	}
	setup(t, c)
	var me struct {
		User      struct{ Login, DisplayName string }
		Household struct{ Name string }
		Role      string
		CSRFToken string
	}
	rec := c.do("GET", "/api/v1/auth/me", nil)
	decode(t, rec, &me)
	if me.User.Login != "marc" || me.Household.Name != "Dom" || me.Role != "admin" || me.CSRFToken != c.csrf {
		t.Fatalf("me: %+v", me)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "Dom", "login": "marc", "password": pw, "displayName": "Marc"})
	flags := map[string]*http.Cookie{}
	for _, ck := range rec.Result().Cookies() {
		flags[ck.Name] = ck
	}
	s, x := flags["mealplan_session"], flags["mealplan_csrf"]
	if s == nil || x == nil {
		t.Fatalf("cookies: %v", flags)
	}
	if !s.HttpOnly || s.SameSite != http.SameSiteLaxMode || s.Path != "/" {
		t.Fatalf("session cookie flags: %+v", s)
	}
	if x.HttpOnly {
		t.Fatal("csrf cookie must be readable by JS")
	}
}

func TestLoginLogoutAndErrors(t *testing.T) {
	e := newEnv(t)
	setup(t, e.client(t))
	c := e.client(t)
	if rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": "wrong-password-123"}); rec.Code != 401 || errCode(t, rec) != "invalid_credentials" {
		t.Fatalf("wrong password: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": " MARC", "password": pw}); rec.Code != 204 {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	c.useCSRF()
	if rec := c.do("POST", "/api/v1/auth/logout", nil); rec.Code != 204 {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("GET", "/api/v1/auth/me", nil); rec.Code != 401 {
		t.Fatalf("after logout: %d", rec.Code)
	}
}

func TestLoginRateLimited(t *testing.T) {
	e := newEnv(t)
	setup(t, e.client(t))
	c := e.client(t)
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": "wrong-password-123"})
	}
	rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": pw})
	if rec.Code != 429 || errCode(t, rec) != "rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d %s %v", rec.Code, rec.Body, rec.Header())
	}
	e.clock.Advance(time.Minute)
	if rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": pw}); rec.Code != 204 {
		t.Fatalf("lock must expire with the clock: %d %s", rec.Code, rec.Body)
	}
}

func TestCSRFRequiredOnAuthenticatedWrites(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	c.csrf = ""
	if rec := c.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 403 || errCode(t, rec) != "csrf" {
		t.Fatalf("missing csrf: %d %s", rec.Code, rec.Body)
	}
	c.csrf = "forged"
	if rec := c.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 403 {
		t.Fatalf("forged csrf: %d", rec.Code)
	}
	c.useCSRF()
	if rec := c.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 201 {
		t.Fatalf("valid csrf: %d %s", rec.Code, rec.Body)
	}
}

func TestPublicPostsRequireJSONContentType(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("login=marc&password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 415 || errCode(t, rec) != "unsupported_media_type" {
		t.Fatalf("expected 415, got %d %s", rec.Code, rec.Body)
	}
}

func TestOversizedBodyAndPassword(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	big := `{"login":"marc","password":"` + strings.Repeat("a", 2<<20) + `"}`
	if rec := c.do("POST", "/api/v1/auth/login", big); rec.Code != 413 || errCode(t, rec) != "too_large" {
		t.Fatalf("big body: %d %s", rec.Code, rec.Body)
	}
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "Dom", "login": "marc", "password": strings.Repeat("a", 200), "displayName": "M"})
	if rec.Code != 422 || errCode(t, rec) != "password_length" {
		t.Fatalf("long password: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/auth/login", `{not json`); rec.Code != 400 || errCode(t, rec) != "bad_request" {
		t.Fatalf("bad json: %d %s", rec.Code, rec.Body)
	}
}

func TestInviteEndToEnd(t *testing.T) {
	e := newEnv(t)
	admin := e.client(t)
	setup(t, admin)

	rec := admin.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"})
	var created struct{ ID, Token, Role, Path string }
	decode(t, rec, &created)
	if rec.Code != 201 || created.Token == "" || created.Path != "/invite/"+created.Token {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var list []struct{ ID, Role string }
	decode(t, admin.do("GET", "/api/v1/auth/invites", nil), &list)
	if len(list) != 1 || list[0].ID != created.ID || strings.Contains(admin.do("GET", "/api/v1/auth/invites", nil).Body.String(), created.Token) {
		t.Fatalf("list must show the invite without its token: %+v", list)
	}

	guest := e.client(t)
	var info struct{ HouseholdName, Role string }
	decode(t, guest.do("GET", "/api/v1/auth/invitations/"+created.Token, nil), &info)
	if info.HouseholdName != "Dom" || info.Role != "member" {
		t.Fatalf("info: %+v", info)
	}
	if rec := guest.do("GET", "/api/v1/auth/invitations/bogus", nil); rec.Code != 404 || errCode(t, rec) != "invite_invalid" {
		t.Fatalf("bogus token: %d %s", rec.Code, rec.Body)
	}
	rec = guest.do("POST", "/api/v1/auth/invitations/"+created.Token+"/accept", map[string]string{"login": "anna", "password": "another-long-password", "displayName": "Anna"})
	if rec.Code != 204 {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	guest.useCSRF()
	var me struct{ Role string }
	decode(t, guest.do("GET", "/api/v1/auth/me", nil), &me)
	if me.Role != "member" {
		t.Fatalf("role: %s", me.Role)
	}
	if rec := guest.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 403 || errCode(t, rec) != "forbidden" {
		t.Fatalf("member creating invite: %d %s", rec.Code, rec.Body)
	}
	var members []struct{ Login, Role string }
	decode(t, guest.do("GET", "/api/v1/household/members", nil), &members)
	if len(members) != 2 {
		t.Fatalf("members: %+v", members)
	}
	if strings.Contains(guest.do("GET", "/api/v1/household/members", nil).Body.String(), "argon2") {
		t.Fatal("password hash leaked")
	}
	if rec := guest.do("POST", "/api/v1/auth/invitations/"+created.Token+"/accept", map[string]string{"login": "ola", "password": "another-long-password", "displayName": "Ola"}); rec.Code != 404 {
		t.Fatalf("reused invite: %d", rec.Code)
	}
}

func TestHouseholdIsolationOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	var created struct{ ID string }
	decode(t, a.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}), &created)

	hash, _ := cheap.Hash("bobs-long-password")
	bob := domain.User{ID: auth.NewID(), Login: "bob", PasswordHash: hash, DisplayName: "Bob", CreatedAt: time.Now()}
	if err := e.st.CreateHouseholdWithAdmin(context.Background(), domain.Household{ID: auth.NewID(), Name: "Inny", CreatedAt: time.Now()}, bob, false); err != nil {
		t.Fatal(err)
	}
	b := e.client(t)
	if rec := b.do("POST", "/api/v1/auth/login", map[string]string{"login": "bob", "password": "bobs-long-password"}); rec.Code != 204 {
		t.Fatalf("bob login: %d %s", rec.Code, rec.Body)
	}
	b.useCSRF()
	if rec := b.do("DELETE", "/api/v1/auth/invites/"+created.ID, nil); rec.Code != 404 {
		t.Fatalf("B deleting A's invite: %d", rec.Code)
	}
	var members []struct{ Login string }
	decode(t, b.do("GET", "/api/v1/household/members", nil), &members)
	if len(members) != 1 || members[0].Login != "bob" {
		t.Fatalf("B members: %+v", members)
	}
	if rec := a.do("DELETE", "/api/v1/auth/invites/"+created.ID, nil); rec.Code != 204 {
		t.Fatalf("A deleting own invite: %d %s", rec.Code, rec.Body)
	}
}

func TestSecurityHeadersAndUnknownAPIPath(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	rec := c.do("GET", "/api/v1/auth/status", nil)
	for h, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(h); got != want {
			t.Errorf("%s = %q, want %q", h, got, want)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("csp: %s", csp)
	}
	rec = c.do("GET", "/api/v1/nope", nil)
	if rec.Code != 404 || errCode(t, rec) != "not_found" {
		t.Fatalf("unknown api path: %d %s", rec.Code, rec.Body)
	}
}
