package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

var now = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	return store.New(storetest.Open(t))
}

func user(id, login string) domain.User {
	return domain.User{ID: id, Login: login, PasswordHash: "h", DisplayName: login, CreatedAt: now}
}

func household(id string) domain.Household {
	return domain.Household{ID: id, Name: "Dom " + id, CreatedAt: now}
}

func TestSetupOnlyWhenNoUsers(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), true); err != nil {
		t.Fatal(err)
	}
	err := s.CreateHouseholdWithAdmin(ctx, household("h2"), user("u2", "eve"), true)
	if !errors.Is(err, store.ErrSetupDone) {
		t.Fatalf("expected ErrSetupDone, got %v", err)
	}
	if n, _ := s.CountUsers(ctx); n != 1 {
		t.Fatalf("users=%d", n)
	}
	hid, role, err := s.FirstMembership(ctx, "u1")
	if err != nil || hid != "h1" || role != domain.RoleAdmin {
		t.Fatalf("membership: %s %s %v", hid, role, err)
	}
}

func TestDuplicateLoginIsConflict(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false); err != nil {
		t.Fatal(err)
	}
	err := s.CreateHouseholdWithAdmin(ctx, household("h2"), user("u2", "marc"), false)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestUserLookups(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	if u, err := s.UserByLogin(ctx, "marc"); err != nil || u.ID != "u1" {
		t.Fatalf("by login: %+v %v", u, err)
	}
	if _, err := s.UserByLogin(ctx, "nobody"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if h, err := s.HouseholdByID(ctx, "h1"); err != nil || h.Name != "Dom h1" {
		t.Fatalf("household: %+v %v", h, err)
	}
}

func inviteFor(id, hid, hash string, ttl time.Duration) domain.Invite {
	return domain.Invite{ID: id, HouseholdID: hid, TokenHash: hash, Role: domain.RoleMember, CreatedBy: "u1", ExpiresAt: now.Add(ttl)}
}

func TestAcceptInviteIsSingleUse(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	if err := s.CreateInvite(ctx, inviteFor("i1", "h1", "hash1", time.Hour)); err != nil {
		t.Fatal(err)
	}
	hid, err := s.AcceptInvite(ctx, "i1", user("u2", "anna"), now)
	if err != nil || hid != "h1" {
		t.Fatalf("accept: %s %v", hid, err)
	}
	role, err := s.RoleOf(ctx, "u2", "h1")
	if err != nil || role != domain.RoleMember {
		t.Fatalf("role: %s %v", role, err)
	}
	if _, err := s.AcceptInvite(ctx, "i1", user("u3", "ola"), now); !errors.Is(err, store.ErrInviteUnusable) {
		t.Fatalf("second accept: %v", err)
	}
}

func TestAcceptInviteRejectsExpired(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	_ = s.CreateInvite(ctx, inviteFor("i1", "h1", "hash1", -time.Minute))
	if _, err := s.AcceptInvite(ctx, "i1", user("u2", "anna"), now); !errors.Is(err, store.ErrInviteUnusable) {
		t.Fatalf("expected ErrInviteUnusable, got %v", err)
	}
}

func TestAcceptInviteConcurrentOnlyOneWins(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	_ = s.CreateInvite(ctx, inviteFor("i1", "h1", "hash1", time.Hour))
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, login := range []string{"anna", "ola"} {
		wg.Add(1)
		go func(i int, login string) {
			defer wg.Done()
			_, results[i] = s.AcceptInvite(ctx, "i1", user("id-"+login, login), now)
		}(i, login)
	}
	wg.Wait()
	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly one winner, results=%v", results)
	}
	if members, _ := s.ListMembers(ctx, "h1"); len(members) != 2 {
		t.Fatalf("members=%d", len(members))
	}
}

func TestInvitesAreScopedToHousehold(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hA"), user("uA", "alice"), false)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hB"), user("uB", "bob"), false)
	_ = s.CreateInvite(ctx, domain.Invite{ID: "iA", HouseholdID: "hA", TokenHash: "tA", Role: domain.RoleMember, CreatedBy: "uA", ExpiresAt: now.Add(time.Hour)})
	if err := s.RevokeInvite(ctx, "hB", "iA"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("household B must not revoke A's invite: %v", err)
	}
	if list, _ := s.ListPendingInvites(ctx, "hB", now); len(list) != 0 {
		t.Fatalf("B sees %d invites", len(list))
	}
	if list, _ := s.ListPendingInvites(ctx, "hA", now); len(list) != 1 {
		t.Fatalf("A sees %d invites", len(list))
	}
	if err := s.RevokeInvite(ctx, "hA", "iA"); err != nil {
		t.Fatal(err)
	}
}

func TestMembersAreScopedToHousehold(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hA"), user("uA", "alice"), false)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hB"), user("uB", "bob"), false)
	members, err := s.ListMembers(ctx, "hB")
	if err != nil || len(members) != 1 || members[0].User.Login != "bob" || members[0].Role != domain.RoleAdmin {
		t.Fatalf("members: %+v %v", members, err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	sess := domain.Session{ID: "s1", UserID: "u1", HouseholdID: "h1", TokenHash: "th", CSRFToken: "c", CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(time.Hour)}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.SessionByTokenHash(ctx, "th")
	if err != nil || got.ID != "s1" || got.CSRFToken != "c" || !got.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	later := now.Add(10 * time.Minute)
	if err := s.TouchSession(ctx, "s1", later); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.SessionByTokenHash(ctx, "th"); !got.LastSeen.Equal(later) {
		t.Fatalf("last_seen not updated: %v", got.LastSeen)
	}
	if err := s.DeleteSession(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionByTokenHash(ctx, "th"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAuditInsert(t *testing.T) {
	s := newStore(t)
	err := s.InsertAudit(context.Background(), domain.AuditEvent{ID: "a1", Type: "login_failed", At: now, Details: "ip=1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
}
