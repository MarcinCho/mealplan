package store

import (
	"context"
	"time"

	"mealplan/internal/domain"
)

func (s *Store) CreateSession(ctx context.Context, x domain.Session) error {
	return s.exec(ctx, s.db, `INSERT INTO session (id, user_id, household_id, token_hash, csrf_token, created_at, last_seen, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		x.ID, x.UserID, x.HouseholdID, x.TokenHash, x.CSRFToken, ts(x.CreatedAt), ts(x.LastSeen), ts(x.ExpiresAt))
}

func (s *Store) SessionByTokenHash(ctx context.Context, hash string) (domain.Session, error) {
	var x domain.Session
	var created, seen, expires int64
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT id, user_id, household_id, token_hash, csrf_token, created_at, last_seen, expires_at FROM session WHERE token_hash = ?`), hash).
		Scan(&x.ID, &x.UserID, &x.HouseholdID, &x.TokenHash, &x.CSRFToken, &created, &seen, &expires)
	if err != nil {
		return domain.Session{}, mapErr(err)
	}
	x.CreatedAt, x.LastSeen, x.ExpiresAt = fromTS(created), fromTS(seen), fromTS(expires)
	return x, nil
}

func (s *Store) TouchSession(ctx context.Context, id string, lastSeen time.Time) error {
	return s.exec(ctx, s.db, `UPDATE session SET last_seen = ? WHERE id = ?`, ts(lastSeen), id)
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	return s.exec(ctx, s.db, `DELETE FROM session WHERE id = ?`, id)
}

func (s *Store) InsertAudit(ctx context.Context, e domain.AuditEvent) error {
	var hid, uid any
	if e.HouseholdID != "" {
		hid = e.HouseholdID
	}
	if e.UserID != "" {
		uid = e.UserID
	}
	return s.exec(ctx, s.db, `INSERT INTO audit_event (id, household_id, user_id, type, at, details) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, hid, uid, e.Type, ts(e.At), e.Details)
}
