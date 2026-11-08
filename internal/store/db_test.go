package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func TestRebind(t *testing.T) {
	pg := &store.DB{Driver: "postgres"}
	if got := pg.Rebind("SELECT ? , ?"); got != "SELECT $1 , $2" {
		t.Fatalf("postgres rebind: %q", got)
	}
	lite := &store.DB{Driver: "sqlite"}
	if got := lite.Rebind("SELECT ?"); got != "SELECT ?" {
		t.Fatalf("sqlite rebind: %q", got)
	}
}

func TestOpenRejectsUnknownDriver(t *testing.T) {
	if _, err := store.Open("mysql", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMigrateIsIdempotentAndCreatesTables(t *testing.T) {
	db := storetest.Open(t) // already migrated once
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, table := range []string{"household", "app_user", "membership", "invite", "session", "audit_event"} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
	var applied int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil || applied != 6 {
		t.Fatalf("schema_migrations: n=%d err=%v", applied, err)
	}
}

func TestOpenSQLiteEnforcesForeignKeysWithPlainDSN(t *testing.T) {
	db, err := store.Open("sqlite", filepath.Join(t.TempDir(), "plain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO membership (user_id, household_id, role) VALUES ('nope', 'nope', 'member')`)
	if err == nil {
		t.Fatal("foreign key violation must be rejected without a custom DSN pragma")
	}
}

func TestMigration0002CreatesCoreTables(t *testing.T) {
	db := storetest.Open(t)
	for _, tbl := range []string{"recipe", "ingredient", "step", "recipe_tag", "plan_entry", "shopping_item"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + tbl).Scan(&n); err != nil {
			t.Errorf("table %s: %v", tbl, err)
		}
	}
}
