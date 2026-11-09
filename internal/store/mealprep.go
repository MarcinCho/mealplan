package store

import (
	"context"
	"database/sql"
	"strings"

	"mealplan/internal/domain"
)

// requireOwned checks inside the transaction that row id of table belongs to the household
// (FKs alone do not check that). An empty id is accepted. table is a package constant.
func (s *Store) requireOwned(ctx context.Context, tx *sql.Tx, table, householdID, id string) error {
	if id == "" {
		return nil
	}
	var n int
	if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM `+table+` WHERE id = ? AND household_id = ?`), id, householdID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- sessions ----

const sessionCols = `id, household_id, date, name, week, cooked, created_at`

func scanSession(r scanner) (domain.PrepSession, error) {
	var p domain.PrepSession
	var cooked int
	var created int64
	if err := r.Scan(&p.ID, &p.HouseholdID, &p.Date, &p.Name, &p.Week, &cooked, &created); err != nil {
		return domain.PrepSession{}, mapErr(err)
	}
	p.Cooked, p.CreatedAt = cooked != 0, fromTS(created)
	return p, nil
}

func (s *Store) InsertPrepSession(ctx context.Context, p domain.PrepSession) error {
	return s.exec(ctx, s.db, `INSERT INTO prep_session (`+sessionCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.HouseholdID, p.Date, p.Name, p.Week, b2i(p.Cooked), ts(p.CreatedAt))
}

func (s *Store) UpdatePrepSession(ctx context.Context, p domain.PrepSession) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`UPDATE prep_session SET date = ?, name = ?, week = ?, cooked = ? WHERE id = ? AND household_id = ?`),
		p.Date, p.Name, p.Week, b2i(p.Cooked), p.ID, p.HouseholdID))
}

func (s *Store) DeletePrepSession(ctx context.Context, householdID, id string) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM prep_session WHERE id = ? AND household_id = ?`), id, householdID))
}

func (s *Store) PrepSessionByID(ctx context.Context, householdID, id string) (domain.PrepSession, error) {
	return scanSession(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+sessionCols+` FROM prep_session WHERE id = ? AND household_id = ?`), id, householdID))
}

func (s *Store) querySessions(ctx context.Context, q string, args ...any) ([]domain.PrepSession, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PrepSession
	for rows.Next() {
		p, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PrepSessions lists the household's sessions, newest date first.
func (s *Store) PrepSessions(ctx context.Context, householdID string) ([]domain.PrepSession, error) {
	return s.querySessions(ctx, `SELECT `+sessionCols+` FROM prep_session WHERE household_id = ? ORDER BY date DESC, created_at DESC, id`, householdID)
}

// PrepSessionsForWeek lists sessions whose week (Monday) column equals week.
func (s *Store) PrepSessionsForWeek(ctx context.Context, householdID, week string) ([]domain.PrepSession, error) {
	return s.querySessions(ctx, `SELECT `+sessionCols+` FROM prep_session WHERE household_id = ? AND week = ? ORDER BY date DESC, created_at DESC, id`, householdID, week)
}

// ---- dishes ----

const dishCols = `id, household_id, session_id, recipe_id, portions, written_off, position`

func scanDish(r scanner) (domain.PrepDish, error) {
	var d domain.PrepDish
	if err := r.Scan(&d.ID, &d.HouseholdID, &d.SessionID, &d.RecipeID, &d.Portions, &d.WrittenOff, &d.Position); err != nil {
		return domain.PrepDish{}, mapErr(err)
	}
	return d, nil
}

// InsertPrepDish appends the dish to its session (position = max+1; the first dish gets 0).
func (s *Store) InsertPrepDish(ctx context.Context, d domain.PrepDish) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.requireOwned(ctx, tx, "prep_session", d.HouseholdID, d.SessionID); err != nil {
			return err
		}
		if err := s.requireOwned(ctx, tx, "recipe", d.HouseholdID, d.RecipeID); err != nil {
			return err
		}
		var pos int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COALESCE(MAX(position) + 1, 0) FROM prep_dish WHERE session_id = ?`), d.SessionID).Scan(&pos); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO prep_dish (`+dishCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			d.ID, d.HouseholdID, d.SessionID, d.RecipeID, d.Portions, d.WrittenOff, pos)
	})
}

// UpdatePrepDish changes portions and written_off; session, recipe and position are fixed.
func (s *Store) UpdatePrepDish(ctx context.Context, d domain.PrepDish) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`UPDATE prep_dish SET portions = ?, written_off = ? WHERE id = ? AND household_id = ?`),
		d.Portions, d.WrittenOff, d.ID, d.HouseholdID))
}

func (s *Store) DeletePrepDish(ctx context.Context, householdID, id string) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM prep_dish WHERE id = ? AND household_id = ?`), id, householdID))
}

func (s *Store) PrepDishByID(ctx context.Context, householdID, id string) (domain.PrepDish, error) {
	return scanDish(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+dishCols+` FROM prep_dish WHERE id = ? AND household_id = ?`), id, householdID))
}

// PrepDishes lists the dishes of the given sessions ordered by session, then position.
func (s *Store) PrepDishes(ctx context.Context, householdID string, sessionIDs []string) ([]domain.PrepDish, error) {
	if len(sessionIDs) == 0 {
		return []domain.PrepDish{}, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(sessionIDs)), ", ")
	args := []any{householdID}
	for _, id := range sessionIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT `+dishCols+` FROM prep_dish WHERE household_id = ? AND session_id IN (`+marks+`) ORDER BY session_id, position, id`), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PrepDish{}
	for rows.Next() {
		d, err := scanDish(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ---- freezer ----

const freezerCols = `id, household_id, dish_id, recipe_id, label, portions, frozen_on`

func scanFreezer(r scanner) (domain.FreezerItem, error) {
	var f domain.FreezerItem
	var dish, recipe sql.NullString
	if err := r.Scan(&f.ID, &f.HouseholdID, &dish, &recipe, &f.Label, &f.Portions, &f.FrozenOn); err != nil {
		return domain.FreezerItem{}, mapErr(err)
	}
	f.DishID, f.RecipeID = dish.String, recipe.String
	return f, nil
}

func (s *Store) checkFreezerRefs(ctx context.Context, tx *sql.Tx, f domain.FreezerItem) error {
	if err := s.requireOwned(ctx, tx, "prep_dish", f.HouseholdID, f.DishID); err != nil {
		return err
	}
	return s.requireOwned(ctx, tx, "recipe", f.HouseholdID, f.RecipeID)
}

func (s *Store) InsertFreezerItem(ctx context.Context, f domain.FreezerItem) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.checkFreezerRefs(ctx, tx, f); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO freezer_item (`+freezerCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			f.ID, f.HouseholdID, nullable(f.DishID), nullable(f.RecipeID), f.Label, f.Portions, f.FrozenOn)
	})
}

func (s *Store) UpdateFreezerItem(ctx context.Context, f domain.FreezerItem) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.checkFreezerRefs(ctx, tx, f); err != nil {
			return err
		}
		return affected(tx.ExecContext(ctx, s.db.Rebind(`UPDATE freezer_item SET dish_id = ?, recipe_id = ?, label = ?, portions = ?, frozen_on = ? WHERE id = ? AND household_id = ?`),
			nullable(f.DishID), nullable(f.RecipeID), f.Label, f.Portions, f.FrozenOn, f.ID, f.HouseholdID))
	})
}

func (s *Store) DeleteFreezerItem(ctx context.Context, householdID, id string) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM freezer_item WHERE id = ? AND household_id = ?`), id, householdID))
}

func (s *Store) FreezerItemByID(ctx context.Context, householdID, id string) (domain.FreezerItem, error) {
	return scanFreezer(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+freezerCols+` FROM freezer_item WHERE id = ? AND household_id = ?`), id, householdID))
}

// FreezerItems lists the household's freezer, oldest first.
func (s *Store) FreezerItems(ctx context.Context, householdID string) ([]domain.FreezerItem, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT `+freezerCols+` FROM freezer_item WHERE household_id = ? ORDER BY frozen_on, id`), householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FreezerItem
	for rows.Next() {
		f, err := scanFreezer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ---- allocations ----

// Allocations returns every prep plan entry of the household, on any date.
func (s *Store) Allocations(ctx context.Context, householdID string) ([]domain.PlanEntry, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT `+planCols+` FROM plan_entry WHERE household_id = ? AND kind = 'prep' ORDER BY date, slot, id`), householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PlanEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
