package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mealplan/internal/domain"
)

// lockPrepDish reads the dish's portions and written-off count inside the transaction and locks
// the row on Postgres; SQLite's single-connection pool serializes transactions anyway.
// ErrNotFound when the dish does not belong to the household.
func (s *Store) lockPrepDish(ctx context.Context, tx *sql.Tx, householdID, id string) (portions, writtenOff int, err error) {
	q := `SELECT portions, written_off FROM prep_dish WHERE id = ? AND household_id = ?`
	if s.db.Driver == "postgres" {
		q += ` FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, s.db.Rebind(q), id, householdID).Scan(&portions, &writtenOff); err != nil {
		return 0, 0, mapErr(err)
	}
	return portions, writtenOff, nil
}

// lockFreezerItem reads the item's portions and dish link inside the transaction (row locked on
// Postgres). ErrNotFound when the item does not belong to the household.
func (s *Store) lockFreezerItem(ctx context.Context, tx *sql.Tx, householdID, id string) (portions int, dishID string, err error) {
	q := `SELECT portions, COALESCE(dish_id, '') FROM freezer_item WHERE id = ? AND household_id = ?`
	if s.db.Driver == "postgres" {
		q += ` FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, s.db.Rebind(q), id, householdID).Scan(&portions, &dishID); err != nil {
		return 0, "", mapErr(err)
	}
	return portions, dishID, nil
}

// dishSupply sums how many portions of the dish are already frozen and planned as prep entries.
// Rows frozen or planned after this call see the dish row locked (Postgres) or the transaction
// serialized (SQLite), so the sums stay valid until commit.
func (s *Store) dishSupply(ctx context.Context, tx *sql.Tx, householdID, dishID string) (frozen, allocated int, err error) {
	if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COALESCE(SUM(portions),0) FROM freezer_item WHERE household_id = ? AND dish_id = ?`), householdID, dishID).Scan(&frozen); err != nil {
		return 0, 0, err
	}
	if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COALESCE(SUM(servings),0) FROM plan_entry WHERE household_id = ? AND kind = 'prep' AND prep_dish_id = ?`), householdID, dishID).Scan(&allocated); err != nil {
		return 0, 0, err
	}
	return frozen, allocated, nil
}

// errPortionSupply marks a write that does not fit the supply still left of a dish.
var errPortionSupply = fmt.Errorf("%w: portions exceed the remaining supply", domain.ErrInvalid)

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

// DeletePrepSessionChecked refuses (ErrInUse) while any dish of the session is planned as a prep
// entry; the check and the delete share one transaction, so a concurrent allocation cannot slip in
// and be silently removed by the cascade.
func (s *Store) DeletePrepSessionChecked(ctx context.Context, householdID, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.lockSessionDishes(ctx, tx, householdID, id); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM plan_entry e JOIN prep_dish d ON d.id = e.prep_dish_id WHERE e.household_id = ? AND e.kind = 'prep' AND d.session_id = ?`), householdID, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrInUse
		}
		return affected(tx.ExecContext(ctx, s.db.Rebind(`DELETE FROM prep_session WHERE id = ? AND household_id = ?`), id, householdID))
	})
}

// lockSessionDishes locks the rows of the session's dishes (ErrNotFound when the session is not
// the household's). Those are the rows a concurrent prep allocation locks, so Postgres
// serializes the two against each other.
func (s *Store) lockSessionDishes(ctx context.Context, tx *sql.Tx, householdID, sessionID string) error {
	q := `SELECT id FROM prep_dish WHERE session_id = ? AND household_id = ?`
	if s.db.Driver == "postgres" {
		q += ` FOR UPDATE`
	}
	rows, err := tx.QueryContext(ctx, s.db.Rebind(q), sessionID, householdID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
	}
	return rows.Err()
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
// Inside the transaction the dish is locked and re-checked against what is already frozen and
// planned, so a concurrent freeze or allocation cannot be silently over-committed.
func (s *Store) UpdatePrepDish(ctx context.Context, d domain.PrepDish) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, _, err := s.lockPrepDish(ctx, tx, d.HouseholdID, d.ID); err != nil {
			return err
		}
		frozen, allocated, err := s.dishSupply(ctx, tx, d.HouseholdID, d.ID)
		if err != nil {
			return err
		}
		if d.Portions < frozen+allocated {
			return fmt.Errorf("%w: portions cannot be fewer than %d already planned, frozen or written off", domain.ErrInvalid, frozen+allocated)
		}
		return affected(tx.ExecContext(ctx, s.db.Rebind(`UPDATE prep_dish SET portions = ?, written_off = ? WHERE id = ? AND household_id = ?`),
			d.Portions, d.WrittenOff, d.ID, d.HouseholdID))
	})
}

func (s *Store) DeletePrepDish(ctx context.Context, householdID, id string) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM prep_dish WHERE id = ? AND household_id = ?`), id, householdID))
}

// DeletePrepDishChecked refuses (ErrInUse) while meals are planned from the dish; the check and
// the delete share one transaction, so a concurrent allocation cannot slip in and be silently
// removed by the cascade.
func (s *Store) DeletePrepDishChecked(ctx context.Context, householdID, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, _, err := s.lockPrepDish(ctx, tx, householdID, id); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM plan_entry WHERE household_id = ? AND kind = 'prep' AND prep_dish_id = ?`), householdID, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrInUse
		}
		return affected(tx.ExecContext(ctx, s.db.Rebind(`DELETE FROM prep_dish WHERE id = ? AND household_id = ?`), id, householdID))
	})
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
		if f.DishID != "" {
			// Lock the dish and re-check the supply inside the transaction: the service-level
			// check saw the sums before it, so only this re-check is authoritative.
			portions, writtenOff, err := s.lockPrepDish(ctx, tx, f.HouseholdID, f.DishID)
			if err != nil {
				return err
			}
			frozen, allocated, err := s.dishSupply(ctx, tx, f.HouseholdID, f.DishID)
			if err != nil {
				return err
			}
			if f.Portions > portions-writtenOff-frozen-allocated {
				return errPortionSupply
			}
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
		// Re-check a portions growth against the dish inside the transaction; the item itself is
		// locked, so its old portions can be dropped from the dish's frozen sum.
		_, oldDish, err := s.lockFreezerItem(ctx, tx, f.HouseholdID, f.ID)
		if err != nil {
			return err
		}
		if f.DishID != "" {
			portions, writtenOff, err := s.lockPrepDish(ctx, tx, f.HouseholdID, f.DishID)
			if err != nil {
				return err
			}
			if oldDish != "" && oldDish != f.DishID { // keep the abandoned dish's row locked too
				q := `SELECT id FROM prep_dish WHERE id = ? AND household_id = ?`
				if s.db.Driver == "postgres" {
					q += ` FOR UPDATE`
				}
				var id string
				if err := tx.QueryRowContext(ctx, s.db.Rebind(q), oldDish, f.HouseholdID).Scan(&id); err != nil && !errors.Is(err, ErrNotFound) {
					return mapErr(err)
				}
			}
			// frozen parts of the dish other than this item, and the dish's planned portions
			var other, allocated int
			if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COALESCE(SUM(portions),0) FROM freezer_item WHERE household_id = ? AND dish_id = ? AND id <> ?`), f.HouseholdID, f.DishID, f.ID).Scan(&other); err != nil {
				return err
			}
			if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COALESCE(SUM(servings),0) FROM plan_entry WHERE household_id = ? AND kind = 'prep' AND prep_dish_id = ?`), f.HouseholdID, f.DishID).Scan(&allocated); err != nil {
				return err
			}
			// what stays frozen of the dish after this update must fit the supply
			if other+f.Portions+allocated+writtenOff > portions {
				return errPortionSupply
			}
		}
		return affected(tx.ExecContext(ctx, s.db.Rebind(`UPDATE freezer_item SET dish_id = ?, recipe_id = ?, label = ?, portions = ?, frozen_on = ? WHERE id = ? AND household_id = ?`),
			nullable(f.DishID), nullable(f.RecipeID), f.Label, f.Portions, f.FrozenOn, f.ID, f.HouseholdID))
	})
}

func (s *Store) DeleteFreezerItem(ctx context.Context, householdID, id string) error {
	return affected(s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM freezer_item WHERE id = ? AND household_id = ?`), id, householdID))
}

// DeleteFreezerItemChecked refuses (ErrInUse) while meals are planned from the item; the check
// and the delete share one transaction, so a concurrent allocation cannot slip in and be
// silently removed by the cascade.
func (s *Store) DeleteFreezerItemChecked(ctx context.Context, householdID, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, _, err := s.lockFreezerItem(ctx, tx, householdID, id); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM plan_entry WHERE household_id = ? AND kind = 'prep' AND freezer_item_id = ?`), householdID, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrInUse
		}
		return affected(tx.ExecContext(ctx, s.db.Rebind(`DELETE FROM freezer_item WHERE id = ? AND household_id = ?`), id, householdID))
	})
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
