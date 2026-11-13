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

func TestMigration0008SplitsRedOnion(t *testing.T) {
	ctx := context.Background()
	db := storetest.OpenRaw(t)
	if err := db.MigrateTo(ctx, "0007_mealprep"); err != nil {
		t.Fatal(err)
	}
	ex := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, db.Rebind(q), args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	ex(`INSERT INTO household (id, name, created_at, products_seeded) VALUES ('h1', 'Dom', 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('c1', 'h1', 'cebula', 'cebula', 1, 1)`)
	for _, a := range []string{"cebula czerwona", "cebuli czerwonej", "cebula biała"} {
		ex(`INSERT INTO product_alias (household_id, product_id, alias) VALUES ('h1', 'c1', ?)`, a)
	}
	// h2 already has its own red onion product.
	ex(`INSERT INTO household (id, name, created_at, products_seeded) VALUES ('h2', 'Dom2', 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('c2', 'h2', 'cebula', 'cebula', 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('r2', 'h2', 'cebula czerwona', 'cebula czerwona', 1, 1)`)
	ex(`INSERT INTO product_alias (household_id, product_id, alias) VALUES ('h2', 'c2', 'cebula czerwona')`)
	// h3 is not seeded: it must not get the product.
	ex(`INSERT INTO household (id, name, created_at) VALUES ('h3', 'Dom3', 1)`)

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRowContext(ctx, db.Rebind(q), args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	aliasOf := func(h, product, alias string) int {
		return count(`SELECT COUNT(*) FROM product_alias a JOIN product p ON p.id = a.product_id WHERE a.household_id = ? AND p.normalized_name = ? AND a.alias = ?`, h, product, alias)
	}
	if aliasOf("h1", "cebula", "cebula biała") != 1 {
		t.Error("cebula lost its white onion alias")
	}
	if n := count(`SELECT COUNT(*) FROM product_alias a JOIN product p ON p.id = a.product_id WHERE p.normalized_name = 'cebula' AND a.alias LIKE '%czerwon%'`); n != 0 {
		t.Errorf("cebula still has %d red aliases", n)
	}
	if n := count(`SELECT COUNT(*) FROM product WHERE household_id = 'h1' AND normalized_name = 'cebula czerwona'`); n != 1 {
		t.Fatalf("h1 red onion products = %d, want 1", n)
	}
	if aliasOf("h1", "cebula czerwona", "czerwona cebula") != 1 || aliasOf("h1", "cebula czerwona", "cebuli czerwonej") != 1 {
		t.Error("h1 red onion lacks its aliases")
	}
	if n := count(`SELECT COUNT(*) FROM product WHERE household_id = 'h2' AND normalized_name = 'cebula czerwona'`); n != 1 {
		t.Errorf("h2 red onion products = %d, want 1", n)
	}
	if n := count(`SELECT COUNT(*) FROM product WHERE household_id = 'h3'`); n != 0 {
		t.Errorf("unseeded household got %d products", n)
	}
	var key, model string
	if err := db.QueryRowContext(ctx, `SELECT openrouter_key, openrouter_model FROM household WHERE id = 'h1'`).Scan(&key, &model); err != nil || key != "" || model != "" {
		t.Fatalf("openrouter columns: %q %q %v", key, model, err)
	}
}

func TestMigration0009SingularPotato(t *testing.T) {
	ctx := context.Background()
	db := storetest.OpenRaw(t)
	if err := db.MigrateTo(ctx, "0008_assistant"); err != nil {
		t.Fatal(err)
	}
	ex := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, db.Rebind(q), args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// h1 has the old seed product: plural name, singular alias.
	ex(`INSERT INTO household (id, name, created_at, products_seeded) VALUES ('h1', 'Dom', 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('z1', 'h1', 'ziemniaki', 'ziemniaki', 1, 1)`)
	for _, a := range []string{"ziemniak", "ziemniaków", "kartofle"} {
		ex(`INSERT INTO product_alias (household_id, product_id, alias) VALUES ('h1', 'z1', ?)`, a)
	}
	// h2 already has a separate "ziemniak" product: nothing may change.
	ex(`INSERT INTO household (id, name, created_at, products_seeded) VALUES ('h2', 'Dom2', 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('z2', 'h2', 'ziemniaki', 'ziemniaki', 1, 1)`)
	ex(`INSERT INTO product (id, household_id, name, normalized_name, created_at, updated_at) VALUES ('z3', 'h2', 'ziemniak', 'ziemniak', 1, 1)`)

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	name := func(id string) string {
		t.Helper()
		var n string
		if err := db.QueryRowContext(ctx, db.Rebind(`SELECT normalized_name FROM product WHERE id = ?`), id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	aliases := func(id string) map[string]bool {
		t.Helper()
		rows, err := db.QueryContext(ctx, db.Rebind(`SELECT alias FROM product_alias WHERE product_id = ?`), id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				t.Fatal(err)
			}
			out[a] = true
		}
		return out
	}
	if got := name("z1"); got != "ziemniak" {
		t.Fatalf("h1 product name = %q, want ziemniak", got)
	}
	if a := aliases("z1"); !a["ziemniaki"] || a["ziemniak"] || !a["kartofle"] || !a["ziemniaków"] {
		t.Fatalf("h1 aliases = %v", a)
	}
	if name("z2") != "ziemniaki" || name("z3") != "ziemniak" {
		t.Fatal("h2 products must stay as they were")
	}
}
