package store

import (
	"context"
	"database/sql"
	"time"

	"mealplan/internal/domain"
)

func (s *Store) CreateInvite(ctx context.Context, inv domain.Invite) error {
	return s.exec(ctx, s.db, `INSERT INTO invite (id, household_id, token_hash, role, created_by, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.HouseholdID, inv.TokenHash, string(inv.Role), inv.CreatedBy, ts(inv.ExpiresAt))
}

const inviteCols = `id, household_id, token_hash, role, created_by, expires_at, used_at`

func scanInvite(r scanner) (domain.Invite, error) {
	var inv domain.Invite
	var role string
	var expires int64
	var used sql.NullInt64
	if err := r.Scan(&inv.ID, &inv.HouseholdID, &inv.TokenHash, &role, &inv.CreatedBy, &expires, &used); err != nil {
		return domain.Invite{}, mapErr(err)
	}
	inv.Role = domain.Role(role)
	inv.ExpiresAt = fromTS(expires)
	if used.Valid {
		t := fromTS(used.Int64)
		inv.UsedAt = &t
	}
	return inv, nil
}

func (s *Store) InviteByTokenHash(ctx context.Context, hash string) (domain.Invite, error) {
	return scanInvite(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+inviteCols+` FROM invite WHERE token_hash = ?`), hash))
}

func (s *Store) ListPendingInvites(ctx context.Context, householdID string, now time.Time) ([]domain.Invite, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT `+inviteCols+` FROM invite WHERE household_id = ? AND used_at IS NULL AND expires_at > ? ORDER BY expires_at, id`),
		householdID, ts(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// RevokeInvite deletes an unused invite, scoped to the household.
func (s *Store) RevokeInvite(ctx context.Context, householdID, id string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(
		`DELETE FROM invite WHERE id = ? AND household_id = ? AND used_at IS NULL`), id, householdID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AcceptInvite atomically consumes an invite and creates the user and membership.
// It fails with ErrInviteUnusable if the invite is used or expired, and ErrConflict if the login is taken.
func (s *Store) AcceptInvite(ctx context.Context, inviteID string, u domain.User, now time.Time) (string, error) {
	var householdID string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, s.db.Rebind(
			`UPDATE invite SET used_at = ? WHERE id = ? AND used_at IS NULL AND expires_at > ?`),
			ts(now), inviteID, ts(now))
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrInviteUnusable
		}
		var role string
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT household_id, role FROM invite WHERE id = ?`), inviteID).
			Scan(&householdID, &role); err != nil {
			return mapErr(err)
		}
		if err := s.insertUser(ctx, tx, u); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO membership (user_id, household_id, role) VALUES (?, ?, ?)`, u.ID, householdID, role)
	})
	if err != nil {
		return "", err
	}
	return householdID, nil
}
