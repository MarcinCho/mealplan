package store

import (
	"context"
	"database/sql"
	"time"

	"mealplan/internal/domain"
)

const planCols = `id, household_id, date, slot, recipe_id, free_text, servings, cook_user_id`

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanEntry(r scanner) (domain.PlanEntry, error) {
	var e domain.PlanEntry
	var slot string
	var recipe, cook sql.NullString
	if err := r.Scan(&e.ID, &e.HouseholdID, &e.Date, &slot, &recipe, &e.FreeText, &e.Servings, &cook); err != nil {
		return domain.PlanEntry{}, mapErr(err)
	}
	e.Slot = domain.Slot(slot)
	e.RecipeID, e.CookUserID = recipe.String, cook.String
	return e, nil
}

// checkRefs verifies, inside the transaction, that the recipe and the cook belong to the household.
func (s *Store) checkRefs(ctx context.Context, tx *sql.Tx, e domain.PlanEntry) error {
	if e.RecipeID != "" {
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM recipe WHERE id = ? AND household_id = ?`), e.RecipeID, e.HouseholdID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
	}
	if e.CookUserID != "" {
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM membership WHERE user_id = ? AND household_id = ?`), e.CookUserID, e.HouseholdID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
	}
	return nil
}

func (s *Store) insertEntry(ctx context.Context, tx *sql.Tx, e domain.PlanEntry) error {
	if err := s.checkRefs(ctx, tx, e); err != nil {
		return err
	}
	return s.exec(ctx, tx, `INSERT INTO plan_entry (`+planCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.HouseholdID, e.Date, string(e.Slot), nullable(e.RecipeID), e.FreeText, e.Servings, nullable(e.CookUserID))
}

func (s *Store) InsertPlanEntry(ctx context.Context, e domain.PlanEntry) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return s.insertEntry(ctx, tx, e) })
}

func (s *Store) UpdatePlanEntry(ctx context.Context, e domain.PlanEntry) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.checkRefs(ctx, tx, e); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, s.db.Rebind(
			`UPDATE plan_entry SET date = ?, slot = ?, recipe_id = ?, free_text = ?, servings = ?, cook_user_id = ? WHERE id = ? AND household_id = ?`),
			e.Date, string(e.Slot), nullable(e.RecipeID), e.FreeText, e.Servings, nullable(e.CookUserID), e.ID, e.HouseholdID)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) DeletePlanEntry(ctx context.Context, householdID, id string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM plan_entry WHERE id = ? AND household_id = ?`), id, householdID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) PlanEntryByID(ctx context.Context, householdID, id string) (domain.PlanEntry, error) {
	return scanEntry(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+planCols+` FROM plan_entry WHERE id = ? AND household_id = ?`), id, householdID))
}

// PlanEntries returns entries with fromDate <= date <= toDate (YYYY-MM-DD compares lexically).
func (s *Store) PlanEntries(ctx context.Context, householdID, fromDate, toDate string) ([]domain.PlanEntry, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT `+planCols+` FROM plan_entry WHERE household_id = ? AND date >= ? AND date <= ? ORDER BY date, slot, id`),
		householdID, fromDate, toDate)
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

// weekBounds widens lo..hi to whole Monday..Sunday weeks; unparsable dates are returned unchanged.
func weekBounds(lo, hi string) (string, string) {
	l, err1 := time.Parse("2006-01-02", lo)
	h, err2 := time.Parse("2006-01-02", hi)
	if err1 != nil || err2 != nil {
		return lo, hi
	}
	l = l.AddDate(0, 0, -((int(l.Weekday()) + 6) % 7))
	h = h.AddDate(0, 0, 6-((int(h.Weekday())+6)%7))
	return l.Format("2006-01-02"), h.Format("2006-01-02")
}

// CopyPlanEntries inserts the batch atomically. It refuses with ErrConflict when the household
// already has entries in any calendar week (Monday..Sunday) the batch touches.
func (s *Store) CopyPlanEntries(ctx context.Context, householdID string, entries []domain.PlanEntry) error {
	if len(entries) == 0 {
		return nil
	}
	lo, hi := entries[0].Date, entries[0].Date
	for _, e := range entries {
		if e.Date < lo {
			lo = e.Date
		}
		if e.Date > hi {
			hi = e.Date
		}
	}
	lo, hi = weekBounds(lo, hi)
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM plan_entry WHERE household_id = ? AND date >= ? AND date <= ?`), householdID, lo, hi).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
		for _, e := range entries {
			e.HouseholdID = householdID
			if err := s.insertEntry(ctx, tx, e); err != nil {
				return err
			}
		}
		return nil
	})
}
