package store

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies pending migrations in filename order, each in its own transaction.
// Migration files are split on ";", so they must not contain ";" inside strings or comments.
func (d *DB) Migrate(ctx context.Context) error { return d.migrateTo(ctx, "") }

// migrateTo applies pending migrations; a non-empty last stops after applying that version.
func (d *DB) migrateTo(ctx context.Context, last string) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at BIGINT NOT NULL)`); err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		version := strings.TrimSuffix(e.Name(), ".sql")
		var n int
		if err := d.QueryRowContext(ctx, d.Rebind(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`), version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range strings.Split(string(body), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %s: %w", version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, d.Rebind(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`), version, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if last != "" && version == last {
			return nil
		}
	}
	return nil
}
