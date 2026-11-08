package store

import (
	"context"
	"database/sql"

	"mealplan/internal/domain"
)

const itemCols = `id, household_id, week, name, normalized_name, quantity, unit, checked, manual, have_quantity, moved`

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func qty(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func scanItem(r scanner) (domain.ShoppingItem, error) {
	var it domain.ShoppingItem
	var q, have sql.NullFloat64
	var checked, manual, moved int
	if err := r.Scan(&it.ID, &it.HouseholdID, &it.Week, &it.Name, &it.NormalizedName, &q, &it.Unit, &checked, &manual, &have, &moved); err != nil {
		return domain.ShoppingItem{}, mapErr(err)
	}
	if q.Valid {
		v := q.Float64
		it.Quantity = &v
	}
	it.Checked, it.Manual, it.Moved = checked != 0, manual != 0, moved != 0
	it.HaveQuantity = pf(have)
	return it, nil
}

func (s *Store) insertItem(ctx context.Context, e execer, it domain.ShoppingItem) error {
	return s.exec(ctx, e, `INSERT INTO shopping_item (`+itemCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.ID, it.HouseholdID, it.Week, it.Name, it.NormalizedName, qty(it.Quantity), it.Unit, b2i(it.Checked), b2i(it.Manual), nf(it.HaveQuantity), b2i(it.Moved))
}

func (s *Store) InsertShoppingItem(ctx context.Context, it domain.ShoppingItem) error {
	return s.insertItem(ctx, s.db, it)
}

func (s *Store) ShoppingItems(ctx context.Context, householdID, week string) ([]domain.ShoppingItem, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT `+itemCols+` FROM shopping_item WHERE household_id = ? AND week = ? ORDER BY checked, normalized_name, unit, id`), householdID, week)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ShoppingItem
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) ShoppingItemByID(ctx context.Context, householdID, id string) (domain.ShoppingItem, error) {
	return scanItem(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+itemCols+` FROM shopping_item WHERE id = ? AND household_id = ?`), id, householdID))
}

func (s *Store) UpdateShoppingItem(ctx context.Context, it domain.ShoppingItem) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(
		`UPDATE shopping_item SET name = ?, normalized_name = ?, quantity = ?, unit = ?, checked = ? WHERE id = ? AND household_id = ?`),
		it.Name, it.NormalizedName, qty(it.Quantity), it.Unit, b2i(it.Checked), it.ID, it.HouseholdID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteShoppingItem(ctx context.Context, householdID, id string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM shopping_item WHERE id = ? AND household_id = ?`), id, householdID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplaceGeneratedItems swaps all non-manual items of the week for the given ones, atomically.
func (s *Store) ReplaceGeneratedItems(ctx context.Context, householdID, week string, items []domain.ShoppingItem) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.exec(ctx, tx, `DELETE FROM shopping_item WHERE household_id = ? AND week = ? AND manual = 0`, householdID, week); err != nil {
			return err
		}
		for _, it := range items {
			it.HouseholdID, it.Week = householdID, week
			if err := s.insertItem(ctx, tx, it); err != nil {
				return err
			}
		}
		return nil
	})
}

// RecipesByIDs loads full recipes for the household; foreign and unknown ids are silently skipped.
func (s *Store) RecipesByIDs(ctx context.Context, householdID string, ids []string) (map[string]domain.Recipe, error) {
	out := map[string]domain.Recipe{}
	for _, id := range ids {
		r, err := s.RecipeByID(ctx, householdID, id)
		if err == ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, nil
}
