package store

import (
	"context"
	"database/sql"

	"mealplan/internal/domain"
)

// AddMember creates a user and their membership atomically.
func (s *Store) AddMember(ctx context.Context, u domain.User, householdID string, role domain.Role) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.insertUser(ctx, tx, u); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO membership (user_id, household_id, role) VALUES (?, ?, ?)`, u.ID, householdID, string(role))
	})
}

// adminsExcluding counts the household's admins other than userID.
func (s *Store) adminsExcluding(ctx context.Context, tx *sql.Tx, householdID, userID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, s.db.Rebind(
		`SELECT COUNT(*) FROM membership WHERE household_id = ? AND role = 'admin' AND user_id <> ?`), householdID, userID).Scan(&n)
	return n, err
}

// lockAdmins serialises concurrent role changes on Postgres by locking the household's admin
// rows; SQLite needs no lock because its single connection serialises transactions.
func (s *Store) lockAdmins(ctx context.Context, tx *sql.Tx, householdID string) error {
	if s.db.Driver != "postgres" {
		return nil
	}
	rows, err := tx.QueryContext(ctx, s.db.Rebind(
		`SELECT user_id FROM membership WHERE household_id = ? AND role = 'admin' FOR UPDATE`), householdID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func (s *Store) memberRole(ctx context.Context, tx *sql.Tx, householdID, userID string) (domain.Role, error) {
	var role string
	err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT role FROM membership WHERE household_id = ? AND user_id = ?`), householdID, userID).Scan(&role)
	if err != nil {
		return "", mapErr(err)
	}
	return domain.Role(role), nil
}

// SetRole changes a member's role; the household must keep at least one admin.
func (s *Store) SetRole(ctx context.Context, householdID, userID string, role domain.Role) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.lockAdmins(ctx, tx, householdID); err != nil {
			return err
		}
		cur, err := s.memberRole(ctx, tx, householdID, userID)
		if err != nil {
			return err
		}
		if cur == domain.RoleAdmin && role != domain.RoleAdmin {
			others, err := s.adminsExcluding(ctx, tx, householdID, userID)
			if err != nil {
				return err
			}
			if others == 0 {
				return ErrLastAdmin
			}
		}
		return s.exec(ctx, tx, `UPDATE membership SET role = ? WHERE household_id = ? AND user_id = ?`, string(role), householdID, userID)
	})
}

// RemoveMember ends a membership: it deletes the user's sessions in the household and clears
// them as cook from the household's plan. The user account itself is kept.
func (s *Store) RemoveMember(ctx context.Context, householdID, userID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.lockAdmins(ctx, tx, householdID); err != nil {
			return err
		}
		cur, err := s.memberRole(ctx, tx, householdID, userID)
		if err != nil {
			return err
		}
		if cur == domain.RoleAdmin {
			others, err := s.adminsExcluding(ctx, tx, householdID, userID)
			if err != nil {
				return err
			}
			if others == 0 {
				return ErrLastAdmin
			}
		}
		for _, q := range []string{
			`UPDATE plan_entry SET cook_user_id = NULL WHERE household_id = ? AND cook_user_id = ?`,
			`DELETE FROM session WHERE household_id = ? AND user_id = ?`,
			`DELETE FROM membership WHERE household_id = ? AND user_id = ?`,
		} {
			if err := s.exec(ctx, tx, q, householdID, userID); err != nil {
				return err
			}
		}
		return nil
	})
}
