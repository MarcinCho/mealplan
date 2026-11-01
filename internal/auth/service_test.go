package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
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

func TestLoginHugePasswordIsRejectedCheaply(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	_, err := svc.Login(context.Background(), "marc", strings.Repeat("a", 5000), "10.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
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
