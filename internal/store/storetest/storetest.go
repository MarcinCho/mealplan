// Package storetest opens a fresh migrated database for tests.
// SQLite by default; set MEALPLAN_TEST_DRIVER=postgres and MEALPLAN_TEST_DSN for Postgres.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"mealplan/internal/store"
)

func Open(t *testing.T) *store.DB {
	t.Helper()
	var db *store.DB
	var err error
	if os.Getenv("MEALPLAN_TEST_DRIVER") == "postgres" {
		db, err = openPostgres(t)
	} else {
		dsn := "file:" + filepath.Join(t.TempDir(), "t.db") + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
		db, err = store.Open("sqlite", dsn)
	}
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func openPostgres(t *testing.T) (*store.DB, error) {
	base := os.Getenv("MEALPLAN_TEST_DSN")
	if base == "" {
		t.Skip("MEALPLAN_TEST_DSN not set")
	}
	admin, err := store.Open("postgres", base)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	schema := "t_" + hex.EncodeToString(buf)
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = admin.Close()
	})
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return store.Open("postgres", u.String())
}
