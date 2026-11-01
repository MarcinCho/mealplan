package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func newTestService(t *testing.T) (*Service, *store.Store, *fakeClock) {
	t.Helper()
	st := store.New(storetest.Open(t))
	clock := newClock()
	svc, err := NewService(st, Config{
		SessionIdle: time.Hour, SessionMax: 24 * time.Hour, InviteTTL: 72 * time.Hour, Hasher: cheap,
	}, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, clock
}

const goodPassword = "correct-horse-battery"

func setupAdmin(t *testing.T, svc *Service) SessionGrant {
	t.Helper()
	g, err := svc.Setup(context.Background(), SetupInput{HouseholdName: "Dom", Login: "marc", Password: goodPassword, DisplayName: "Marc"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	return g
}

func TestSetupCreatesAdminAndSession(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t)
	if need, _ := svc.SetupRequired(ctx); !need {
		t.Fatal("setup should be required on empty database")
	}
	g := setupAdmin(t, svc)
	p, err := svc.Authenticate(ctx, g.Token)
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != domain.RoleAdmin || p.HouseholdName != "Dom" || p.DisplayName != "Marc" || p.CSRFToken != g.CSRFToken {
		t.Fatalf("principal: %+v", p)
	}
	if need, _ := svc.SetupRequired(ctx); need {
		t.Fatal("setup must not be required afterwards")
	}
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: "X", Login: "eve", Password: goodPassword, DisplayName: "Eve"}); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second setup: %v", err)
	}
}

func TestSetupValidatesInput(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: "Dom", Login: "marc", Password: "short", DisplayName: "M"}); !errors.Is(err, ErrPasswordLength) {
		t.Fatalf("short password: %v", err)
	}
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: "Dom", Login: "a b", Password: goodPassword, DisplayName: "M"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad login: %v", err)
	}
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: " ", Login: "marc", Password: goodPassword, DisplayName: "M"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty household: %v", err)
	}
}

func TestLoginNormalizesLogin(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	if _, err := svc.Login(context.Background(), "  MARC ", goodPassword, "10.0.0.1"); err != nil {
		t.Fatalf("login with spaces and capitals: %v", err)
	}
}

func TestLoginFailuresLookAlike(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	ctx := context.Background()
	_, e1 := svc.Login(ctx, "marc", "wrong-password-123", "10.0.0.1")
	_, e2 := svc.Login(ctx, "nobody", "wrong-password-123", "10.0.0.2")
	if !errors.Is(e1, ErrInvalidCredentials) || !errors.Is(e2, ErrInvalidCredentials) {
		t.Fatalf("errors: %v / %v", e1, e2)
	}
}

func TestLoginOversizedPasswordSkipsVerify(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	calls := 0
	real := svc.verify
	svc.verify = func(p, e string) (bool, error) { calls++; return real(p, e) }
	ctx := context.Background()

	if _, err := svc.Login(ctx, "marc", "wrong-password-123", "10.0.0.1"); !errors.Is(err, ErrInvalidCredentials) || calls != 1 {
		t.Fatalf("normal wrong password: err=%v calls=%d", err, calls)
	}
	for name, pw := range map[string]string{"5000 chars": strings.Repeat("a", 5000), "129 runes": strings.Repeat("ż", MaxPasswordLen+1)} {
		if _, err := svc.Login(ctx, "marc", pw, "10.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("%s: %v", name, err)
		}
		if calls != 1 {
			t.Fatalf("%s: verify was called (calls=%d)", name, calls)
		}
	}
	if _, err := svc.Login(ctx, "marc", strings.Repeat("ż", MaxPasswordLen), "10.0.0.1"); !errors.Is(err, ErrInvalidCredentials) || calls != 2 {
		t.Fatalf("boundary password must reach verify: err=%v calls=%d", err, calls)
	}
}

func TestLoginLockoutAndRecovery(t *testing.T) {
	svc, _, clock := newTestService(t)
	setupAdmin(t, svc)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(ctx, "marc", "wrong-password-123", "10.0.0.1")
	}
	_, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.1")
	var rl *RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter <= 0 {
		t.Fatalf("expected RateLimitedError, got %v", err)
	}
	clock.Advance(time.Minute)
	if _, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.1"); err != nil {
		t.Fatalf("lockout must expire: %v", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	svc, _, clock := newTestService(t)
	ctx := context.Background()
	g := setupAdmin(t, svc)

	clock.Advance(30 * time.Minute)
	if _, err := svc.Authenticate(ctx, g.Token); err != nil {
		t.Fatalf("active session: %v", err)
	}
	clock.Advance(40 * time.Minute) // 40 min since last activity, 70 since start: idle limit is 60
	if _, err := svc.Authenticate(ctx, g.Token); err != nil {
		t.Fatalf("activity must extend idle window: %v", err)
	}
	clock.Advance(61 * time.Minute)
	if _, err := svc.Authenticate(ctx, g.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("idle expiry: %v", err)
	}
}

func TestSessionAbsoluteExpiry(t *testing.T) {
	svc, _, clock := newTestService(t)
	ctx := context.Background()
	g := setupAdmin(t, svc)
	for i := 0; i < 25; i++ { // stay active every 59 minutes, past the 24h cap
		clock.Advance(59 * time.Minute)
		if _, err := svc.Authenticate(ctx, g.Token); err != nil {
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("session survived past absolute expiry")
}

func TestLogoutInvalidatesSession(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	g := setupAdmin(t, svc)
	if err := svc.Logout(ctx, g.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, g.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
	if _, err := svc.Authenticate(ctx, ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty token: %v", err)
	}
}

func TestInviteFlow(t *testing.T) {
	svc, st, clock := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)

	raw, inv, err := svc.CreateInvite(ctx, admin, domain.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InviteByTokenHash(ctx, raw); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("raw token must not be stored")
	}
	if _, err := st.InviteByTokenHash(ctx, HashToken(raw)); err != nil {
		t.Fatalf("hashed token must be stored: %v", err)
	}
	info, err := svc.InviteInfo(ctx, raw)
	if err != nil || info.HouseholdName != "Dom" || info.Role != domain.RoleMember {
		t.Fatalf("info: %+v %v", info, err)
	}

	g, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "Anna", Password: "another-long-password", DisplayName: "Anna"})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	member, err := svc.Authenticate(ctx, g.Token)
	if err != nil || member.Role != domain.RoleMember || member.HouseholdID != admin.HouseholdID {
		t.Fatalf("member: %+v %v", member, err)
	}
	if _, _, err := svc.CreateInvite(ctx, member, domain.RoleMember); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member creating invite: %v", err)
	}
	if _, err := svc.ListInvites(ctx, member); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member listing invites: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "ola", Password: "another-long-password", DisplayName: "Ola"}); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("reuse: %v", err)
	}
	if _, err := svc.InviteInfo(ctx, raw); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("info after use: %v", err)
	}
	if list, _ := svc.ListInvites(ctx, admin); len(list) != 0 {
		t.Fatalf("used invite must not be pending, got %d", len(list))
	}

	raw2, _, _ := svc.CreateInvite(ctx, admin, domain.RoleMember)
	clock.Advance(73 * time.Hour)
	if _, err := svc.AcceptInvite(ctx, raw2, AcceptInput{Login: "ola", Password: "another-long-password", DisplayName: "Ola"}); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("expired: %v", err)
	}
	_ = inv
}

func TestAcceptInviteRejectsTakenLoginAndUnknownToken(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	raw, _, _ := svc.CreateInvite(ctx, admin, domain.RoleMember)
	if _, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "marc", Password: "another-long-password", DisplayName: "X"}); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("taken login: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, "no-such-token", AcceptInput{Login: "ola", Password: "another-long-password", DisplayName: "Ola"}); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("unknown token: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "ola", Password: "short", DisplayName: "Ola"}); !errors.Is(err, ErrPasswordLength) {
		t.Fatalf("short password: %v", err)
	}
	if _, _, err := svc.CreateInvite(ctx, admin, domain.Role("root")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad role: %v", err)
	}
}

func TestAcceptInviteConcurrent(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	raw, _, _ := svc.CreateInvite(ctx, admin, domain.RoleMember)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, login := range []string{"anna", "ola"} {
		wg.Add(1)
		go func(i int, login string) {
			defer wg.Done()
			_, errs[i] = svc.AcceptInvite(ctx, raw, AcceptInput{Login: login, Password: "another-long-password", DisplayName: login})
		}(i, login)
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly one success, got %v", errs)
	}
}

func TestHouseholdIsolation(t *testing.T) {
	svc, st, clock := newTestService(t)
	ctx := context.Background()
	adminA, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	_, inv, err := svc.CreateInvite(ctx, adminA, domain.RoleMember)
	if err != nil {
		t.Fatal(err)
	}

	hash, _ := svc.cfg.Hasher.Hash("bobs-long-password")
	bob := domain.User{ID: NewID(), Login: "bob", PasswordHash: hash, DisplayName: "Bob", CreatedAt: clock.Now()}
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: NewID(), Name: "Inny", CreatedAt: clock.Now()}, bob, false); err != nil {
		t.Fatal(err)
	}
	g, err := svc.Login(ctx, "bob", "bobs-long-password", "10.0.0.9")
	if err != nil {
		t.Fatal(err)
	}
	adminB, _ := svc.Authenticate(ctx, g.Token)

	if err := svc.RevokeInvite(ctx, adminB, inv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("B revoking A's invite: %v", err)
	}
	if list, _ := svc.ListInvites(ctx, adminB); len(list) != 0 {
		t.Fatalf("B sees %d of A's invites", len(list))
	}
	members, _ := svc.Members(ctx, adminB)
	if len(members) != 1 || members[0].User.Login != "bob" {
		t.Fatalf("B members: %+v", members)
	}
}

func TestSetupAfterDoneDoesNotHash(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	calls := 0
	real := svc.hash
	svc.hash = func(p string) (string, error) { calls++; return real(p) }
	_, err := svc.Setup(context.Background(), SetupInput{HouseholdName: "X", Login: "eve", Password: goodPassword, DisplayName: "Eve"})
	if !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second setup: %v", err)
	}
	if calls != 0 {
		t.Fatalf("hash was called %d times after setup was done", calls)
	}
}

func TestArgonConcurrencyIsBounded(t *testing.T) {
	svc, _, _ := newTestService(t)
	size := cap(svc.sem)
	if size < 1 || size > 4 {
		t.Fatalf("semaphore size %d out of range", size)
	}
	var inflight, max atomic.Int32
	release := make(chan struct{})
	enter := func() {
		n := inflight.Add(1)
		for {
			m := max.Load()
			if n <= m || max.CompareAndSwap(m, n) {
				break
			}
		}
		<-release
		inflight.Add(-1)
	}
	svc.hash = func(string) (string, error) { enter(); return "h", nil }
	svc.verify = func(string, string) (bool, error) { enter(); return false, nil }

	n := size + 3
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, _ = svc.Login(context.Background(), "nobody", "wrong-password-123", "10.0.0.1")
			} else {
				_, _ = svc.hashPassword(context.Background(), goodPassword)
			}
		}(i)
	}
	deadline := time.Now().Add(5 * time.Second)
	for inflight.Load() < int32(size) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // give excess goroutines the chance to (wrongly) enter
	if got := inflight.Load(); got != int32(size) {
		t.Fatalf("in flight %d, want exactly %d", got, size)
	}
	close(release)
	wg.Wait()
	if got := max.Load(); got > int32(size) {
		t.Fatalf("max concurrent %d exceeds bound %d", got, size)
	}
}

func TestArgonSlotHonorsContext(t *testing.T) {
	svc, _, _ := newTestService(t)
	for i := 0; i < cap(svc.sem); i++ {
		svc.sem <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.hashPassword(ctx, goodPassword); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestLockedAccountDoesNotLockOtherAccountOnSameIP(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	setupAdmin(t, svc)
	mustInvite(t, svc, ctx)
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(ctx, "marc", "wrong-password-123", "10.0.0.9")
	}
	var rl *RateLimitedError
	if _, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.9"); !errors.As(err, &rl) {
		t.Fatalf("account A must be locked, got %v", err)
	}
	if _, err := svc.Login(ctx, "anna", goodPassword, "10.0.0.9"); err != nil {
		t.Fatalf("account B from the same IP must still log in: %v", err)
	}
}

func TestIPLimiterLocksAfterManyDifferentLogins(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	setupAdmin(t, svc)
	for i := 0; i < 30; i++ {
		_, err := svc.Login(ctx, fmt.Sprintf("ghost%d", i), "wrong-password-123", "10.0.0.7")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	var rl *RateLimitedError
	if _, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.7"); !errors.As(err, &rl) {
		t.Fatalf("IP must be locked after 30 failures, got %v", err)
	}
	if _, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.8"); err != nil {
		t.Fatalf("other IP unaffected: %v", err)
	}
}

func TestLoginLockoutReachesFifteenMinuteCap(t *testing.T) {
	svc, _, clock := newTestService(t)
	ctx := context.Background()
	setupAdmin(t, svc)
	var wait time.Duration
	for i := 0; i < 60 && wait != 15*time.Minute; i++ {
		clock.Advance(wait) // let the current lock expire; distinct IPs keep the IP limiter out of the way
		_, _ = svc.Login(ctx, "marc", "wrong-password-123", fmt.Sprintf("10.2.0.%d", i))
		_, wait = svc.loginLimiter.Allow("login:marc")
	}
	if wait != 15*time.Minute {
		t.Fatalf("lock never reached the cap, last=%v", wait)
	}
	var rl *RateLimitedError
	if _, err := svc.Login(ctx, "marc", goodPassword, "10.3.0.1"); !errors.As(err, &rl) || rl.RetryAfter != 15*time.Minute {
		t.Fatalf("service RetryAfter: %v", err)
	}
}

func TestLoginOversizedLoginRejectedWithoutLimiterEntry(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	calls := 0
	real := svc.verify
	svc.verify = func(p, e string) (bool, error) { calls++; return real(p, e) }
	ctx := context.Background()
	long := strings.Repeat("a", 65)
	for i := 0; i < 10; i++ {
		if _, err := svc.Login(ctx, long, goodPassword, "10.0.0.5"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("oversized login: %v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("verify called %d times for an oversized login", calls)
	}
	svc.loginLimiter.mu.Lock()
	n := len(svc.loginLimiter.entries)
	svc.loginLimiter.mu.Unlock()
	if n != 0 {
		t.Fatalf("login limiter has %d entries, want 0", n)
	}
	svc.ipLimiter.mu.Lock()
	e := svc.ipLimiter.entries["ip:10.0.0.5"]
	svc.ipLimiter.mu.Unlock()
	if e == nil || e.fails != 10 {
		t.Fatalf("IP failures must still be counted, entry=%+v", e)
	}
}

func mustInvite(t *testing.T, svc *Service, ctx context.Context) SessionGrant {
	t.Helper()
	p, err := svc.Authenticate(ctx, setupToken(t, svc))
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := svc.CreateInvite(ctx, p, domain.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	g, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "anna", Password: goodPassword, DisplayName: "Anna"})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func setupToken(t *testing.T, svc *Service) string {
	t.Helper()
	g, err := svc.Login(context.Background(), "marc", goodPassword, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	return g.Token
}

func TestRemoveMemberAndRoleAudit(t *testing.T) {
	svc, st, _ := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	raw, _, err := svc.CreateInvite(ctx, admin, domain.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	g, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "mia", Password: goodPassword, DisplayName: "Mia"})
	if err != nil {
		t.Fatal(err)
	}
	member, _ := svc.Authenticate(ctx, g.Token)
	if err := svc.ChangeRole(ctx, member, admin.UserID, domain.RoleMember); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member changing role: %v", err)
	}
	if err := svc.ChangeRole(ctx, admin, member.UserID, domain.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangeRole(ctx, admin, member.UserID, domain.Role("bogus")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad role: %v", err)
	}
	if err := svc.RemoveMember(ctx, admin, member.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, g.Token); err == nil {
		t.Fatal("removed member session must be dead")
	}
	if _, err := svc.Login(ctx, "mia", goodPassword, "1.2.3.4"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("removed member login: %v", err)
	}
	if err := svc.RemoveMember(ctx, admin, admin.UserID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last admin: %v", err)
	}
	for _, typ := range []string{"role_changed", "member_removed"} {
		var n int
		if err := st.DB().QueryRowContext(ctx, st.DB().Rebind(`SELECT COUNT(*) FROM audit_event WHERE type = ?`), typ).Scan(&n); err != nil || n != 1 {
			t.Fatalf("audit %s: n=%d err=%v", typ, n, err)
		}
	}
}
