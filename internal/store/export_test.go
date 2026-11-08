package store

import "context"

// MigrateTo exposes migrateTo to the external tests.
func (d *DB) MigrateTo(ctx context.Context, last string) error { return d.migrateTo(ctx, last) }
