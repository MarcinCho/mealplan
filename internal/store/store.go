package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound       = errors.New("store: not found")
	ErrConflict       = errors.New("store: conflict")
	ErrSetupDone      = errors.New("store: setup already done")
	ErrInviteUnusable = errors.New("store: invite unusable")
	ErrInUse          = errors.New("store: in use")
	ErrLastAdmin      = errors.New("store: last admin")
)

type Store struct{ db *DB }

func New(db *DB) *Store { return &Store{db: db} }

// DB exposes the handle (for tests and maintenance code).
func (s *Store) DB() *DB { return s.db }

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type scanner interface{ Scan(dest ...any) error }

func (s *Store) exec(ctx context.Context, e execer, q string, args ...any) error {
	_, err := e.ExecContext(ctx, s.db.Rebind(q), args...)
	return mapErr(err)
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key") {
		return ErrConflict
	}
	return err
}

func ts(t time.Time) int64 { return t.Unix() }

func fromTS(n int64) time.Time { return time.Unix(n, 0).UTC() }
