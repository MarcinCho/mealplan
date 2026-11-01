package store

import (
	"context"
	"database/sql"

	"mealplan/internal/domain"
)

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_user`).Scan(&n)
	return n, err
}

// CreateHouseholdWithAdmin creates a household and its first admin atomically.
// With onlyIfNoUsers it fails with ErrSetupDone when any user already exists.
func (s *Store) CreateHouseholdWithAdmin(ctx context.Context, h domain.Household, u domain.User, onlyIfNoUsers bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if onlyIfNoUsers {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_user`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return ErrSetupDone
			}
		}
		if err := s.exec(ctx, tx, `INSERT INTO household (id, name, created_at) VALUES (?, ?, ?)`,
			h.ID, h.Name, ts(h.CreatedAt)); err != nil {
			return err
		}
		if err := s.insertUser(ctx, tx, u); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO membership (user_id, household_id, role) VALUES (?, ?, ?)`,
			u.ID, h.ID, string(domain.RoleAdmin))
	})
}

func (s *Store) insertUser(ctx context.Context, e execer, u domain.User) error {
	return s.exec(ctx, e, `INSERT INTO app_user (id, login, password_hash, display_name, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Login, u.PasswordHash, u.DisplayName, ts(u.CreatedAt))
}

func scanUser(r scanner) (domain.User, error) {
	var u domain.User
	var created int64
	if err := r.Scan(&u.ID, &u.Login, &u.PasswordHash, &u.DisplayName, &created); err != nil {
		return domain.User{}, mapErr(err)
	}
	u.CreatedAt = fromTS(created)
	return u, nil
}

const userCols = `id, login, password_hash, display_name, created_at`

func (s *Store) UserByLogin(ctx context.Context, login string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+userCols+` FROM app_user WHERE login = ?`), login))
}

func (s *Store) UserByID(ctx context.Context, id string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+userCols+` FROM app_user WHERE id = ?`), id))
}

func (s *Store) HouseholdByID(ctx context.Context, id string) (domain.Household, error) {
	var h domain.Household
	var created int64
	err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT id, name, created_at FROM household WHERE id = ?`), id).
		Scan(&h.ID, &h.Name, &created)
	if err != nil {
		return domain.Household{}, mapErr(err)
	}
	h.CreatedAt = fromTS(created)
	return h, nil
}

func (s *Store) FirstMembership(ctx context.Context, userID string) (string, domain.Role, error) {
	var hid, role string
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT household_id, role FROM membership WHERE user_id = ? ORDER BY household_id LIMIT 1`), userID).Scan(&hid, &role)
	if err != nil {
		return "", "", mapErr(err)
	}
	return hid, domain.Role(role), nil
}

func (s *Store) RoleOf(ctx context.Context, userID, householdID string) (domain.Role, error) {
	var role string
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT role FROM membership WHERE user_id = ? AND household_id = ?`), userID, householdID).Scan(&role)
	if err != nil {
		return "", mapErr(err)
	}
	return domain.Role(role), nil
}

func (s *Store) ListMembers(ctx context.Context, householdID string) ([]domain.Member, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT u.id, u.login, u.password_hash, u.display_name, u.created_at, m.role
		   FROM membership m JOIN app_user u ON u.id = m.user_id
		  WHERE m.household_id = ? ORDER BY u.display_name, u.id`), householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Member
	for rows.Next() {
		var m domain.Member
		var created int64
		var role string
		if err := rows.Scan(&m.User.ID, &m.User.Login, &m.User.PasswordHash, &m.User.DisplayName, &created, &role); err != nil {
			return nil, err
		}
		m.User.CreatedAt = fromTS(created)
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}
