package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

// seedHousehold creates hA with admin uA and member uM.
func seedMembers(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()
	if err := s.CreateHouseholdWithAdmin(ctx, household("hA"), user("uA", "alice"), false); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateHouseholdWithAdmin(ctx, household("hB"), user("uB", "bob"), false); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, user("uM", "mia"), "hA", domain.RoleMember); err != nil {
		t.Fatal(err)
	}
}

func TestSetRoleAndLastAdminGuard(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedMembers(t, s)
	if err := s.SetRole(ctx, "hA", "uA", domain.RoleMember); !errors.Is(err, store.ErrLastAdmin) {
		t.Fatalf("demoting last admin: %v", err)
	}
	if err := s.SetRole(ctx, "hA", "uM", domain.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRole(ctx, "hA", "uA", domain.RoleMember); err != nil {
		t.Fatalf("demoting one of two admins: %v", err)
	}
	if err := s.SetRole(ctx, "hA", "uB", domain.RoleMember); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("role of foreign member: %v", err)
	}
	if role, _ := s.RoleOf(ctx, "uB", "hB"); role != domain.RoleAdmin {
		t.Fatalf("other household changed: %s", role)
	}
}

func TestRemoveMemberCleansUp(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedMembers(t, s)
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO plan_entry (id, household_id, date, slot, free_text, servings, cook_user_id) VALUES ('e1','hA','2026-11-02','lunch','x',1,'uM')`); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, domain.Session{ID: "s1", UserID: "uM", HouseholdID: "hA", TokenHash: "th", CSRFToken: "c", CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(3600e9)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveMember(ctx, "hA", "uM"); err != nil {
		t.Fatal(err)
	}
	e, err := s.PlanEntryByID(ctx, "hA", "e1")
	if err != nil || e.CookUserID != "" {
		t.Fatalf("entry must stay, cook cleared: %v %+v", err, e)
	}
	if _, err := s.SessionByTokenHash(ctx, "th"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session must be gone: %v", err)
	}
	if _, err := s.RoleOf(ctx, "uM", "hA"); err == nil {
		t.Fatal("membership must be gone")
	}
}

func TestRemoveLastAdminAndForeignMember(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedMembers(t, s)
	if err := s.RemoveMember(ctx, "hA", "uA"); !errors.Is(err, store.ErrLastAdmin) {
		t.Fatalf("removing last admin: %v", err)
	}
	if err := s.RemoveMember(ctx, "hA", "uB"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("removing foreign member: %v", err)
	}
	if _, err := s.RoleOf(ctx, "uB", "hB"); err != nil {
		t.Fatalf("foreign membership damaged: %v", err)
	}
}
