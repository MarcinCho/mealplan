package store_test

import (
	"context"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func TestMigration0007KeepsRows(t *testing.T) {
	ctx := context.Background()
	db := storetest.OpenRaw(t)
	if err := db.MigrateTo(ctx, "0006_pantry"); err != nil {
		t.Fatal(err)
	}
	ex := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, db.Rebind(q), args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	ex(`INSERT INTO household (id, name, created_at) VALUES ('h1', 'Dom', 1)`)
	ex(`INSERT INTO app_user (id, login, password_hash, display_name, created_at) VALUES ('u1', 'ala', 'x', 'Ala', 1)`)
	ex(`INSERT INTO recipe (id, household_id, title, base_servings, created_at, updated_at) VALUES ('r1', 'h1', 'Zupa', 2, 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('p1', 'h1', 'mąka', 'mąka', 1, 1)`)
	ex(`INSERT INTO plan_entry (id, household_id, date, slot, recipe_id, free_text, servings, cook_user_id, done) VALUES ('e1', 'h1', '2026-11-02', 'dinner', 'r1', '', 3, 'u1', 1)`)
	ex(`INSERT INTO plan_entry (id, household_id, date, slot, recipe_id, free_text, servings, cook_user_id, done) VALUES ('e2', 'h1', '2026-11-03', 'lunch', NULL, 'Resztki', 1, NULL, 0)`)
	ex(`INSERT INTO pantry_item (id, household_id, product_id, quantity, unit, expires_on, staple, updated_at) VALUES ('i1', 'h1', 'p1', 100, 'g', '', 0, 1)`)
	ex(`INSERT INTO pantry_movement (household_id, entry_id, product_id, unit, amount) VALUES ('h1', 'e1', 'p1', 'g', -40)`)

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := store.New(db)
	e1, err := s.PlanEntryByID(ctx, "h1", "e1")
	want1 := domain.PlanEntry{ID: "e1", HouseholdID: "h1", Date: "2026-11-02", Slot: domain.SlotDinner, RecipeID: "r1", Servings: 3, CookUserID: "u1", Done: true, Kind: domain.EntryMeal}
	if err != nil || e1 != want1 {
		t.Fatalf("e1: %v %+v", err, e1)
	}
	e2, err := s.PlanEntryByID(ctx, "h1", "e2")
	want2 := domain.PlanEntry{ID: "e2", HouseholdID: "h1", Date: "2026-11-03", Slot: domain.SlotLunch, FreeText: "Resztki", Servings: 1, Kind: domain.EntryMeal}
	if err != nil || e2 != want2 {
		t.Fatalf("e2: %v %+v", err, e2)
	}
	if err := s.RestoreForEntry(ctx, "h1", "e1"); err != nil {
		t.Fatal(err)
	}
	it, err := s.PantryItemByID(ctx, "h1", "i1")
	if err != nil || it.Quantity == nil || *it.Quantity != 140 {
		t.Fatalf("pantry after restore: %v %+v", err, it)
	}
	if n, err := s.PortionsPerMeal(ctx, "h1"); err != nil || n != 2 {
		t.Fatalf("portions per meal: %d %v", n, err)
	}
	ex(`INSERT INTO plan_entry (id, household_id, date, slot, free_text, servings, kind) VALUES ('e3', 'h1', '2026-11-04', 'dinner', '', 1, 'out')`)
	if _, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO plan_entry (id, household_id, date, slot, recipe_id, free_text, servings, kind) VALUES ('e4', 'h1', '2026-11-04', 'dinner', 'r1', 'x', 1, 'meal')`)); err == nil {
		t.Fatal("meal with recipe and free text must fail the CHECK")
	}
}
