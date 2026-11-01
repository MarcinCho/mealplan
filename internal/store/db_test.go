package store_test

import (
	"context"
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
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("schema_migrations: n=%d err=%v", applied, err)
	}
}
