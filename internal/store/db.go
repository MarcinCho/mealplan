// Package store is the persistence layer. SQL is portable between SQLite and Postgres.
package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Driver string // "sqlite" or "postgres"
}

func Open(driver, dsn string) (*DB, error) {
	var name string
	switch driver {
	case "sqlite":
		name = "sqlite"
	case "postgres":
		name = "pgx"
	default:
		return nil, fmt.Errorf("store: unknown driver %q", driver)
	}
	if driver == "sqlite" {
		dsn = sqliteDSN(dsn)
	}
	sdb, err := sql.Open(name, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		sdb.SetMaxOpenConns(1) // SQLite allows a single writer; avoids SQLITE_BUSY
	}
	if err := sdb.Ping(); err != nil {
		_ = sdb.Close()
		return nil, err
	}
	return &DB{DB: sdb, Driver: driver}, nil
}

// Rebind converts "?" placeholders to "$n" for Postgres.
func (d *DB) Rebind(q string) string {
	if d.Driver != "postgres" {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// sqliteDSN makes sure foreign keys are enforced; SQLite leaves them off by default and the
// setting is per connection, so it has to travel with the DSN.
func sqliteDSN(dsn string) string {
	if strings.Contains(dsn, "foreign_keys") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=foreign_keys(1)"
}
