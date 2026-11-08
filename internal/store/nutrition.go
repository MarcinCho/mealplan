package store

import (
	"context"
	"database/sql"

	"mealplan/internal/domain"
)

// NutritionSettings returns the user's settings; no row means disabled with empty goals.
func (s *Store) NutritionSettings(ctx context.Context, householdID, userID string) (domain.NutritionSettings, error) {
	out := domain.NutritionSettings{UserID: userID, HouseholdID: householdID}
	var en int
	var k, p, c, f sql.NullFloat64
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT enabled, goal_kcal, goal_protein, goal_carbs, goal_fat FROM nutrition_settings WHERE household_id = ? AND user_id = ?`),
		householdID, userID).Scan(&en, &k, &p, &c, &f)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Enabled = en != 0
	out.Goal = domain.Macros{Kcal: pf(k), Protein: pf(p), Carbs: pf(c), Fat: pf(f)}
	return out, nil
}

func (s *Store) SaveNutritionSettings(ctx context.Context, n domain.NutritionSettings) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.exec(ctx, tx, `DELETE FROM nutrition_settings WHERE user_id = ? AND household_id = ?`, n.UserID, n.HouseholdID); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO nutrition_settings (user_id, household_id, enabled, goal_kcal, goal_protein, goal_carbs, goal_fat) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			n.UserID, n.HouseholdID, b2i(n.Enabled), nf(n.Goal.Kcal), nf(n.Goal.Protein), nf(n.Goal.Carbs), nf(n.Goal.Fat))
	})
}

func (s *Store) AddFoodLog(ctx context.Context, e domain.FoodLogEntry) error {
	return s.exec(ctx, s.db, `INSERT INTO food_log (id, household_id, user_id, date, product_id, quantity, unit, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.HouseholdID, e.UserID, e.Date, e.ProductID, e.Quantity, e.Unit, ts(e.CreatedAt))
}

func (s *Store) DeleteFoodLog(ctx context.Context, householdID, userID, id string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM food_log WHERE id = ? AND household_id = ? AND user_id = ?`), id, householdID, userID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// FoodLog returns the user's entries with from <= date <= to (YYYY-MM-DD), oldest first.
func (s *Store) FoodLog(ctx context.Context, householdID, userID, from, to string) ([]domain.FoodLogEntry, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT id, household_id, user_id, date, product_id, quantity, unit, created_at FROM food_log
		 WHERE household_id = ? AND user_id = ? AND date >= ? AND date <= ? ORDER BY date, created_at, id`),
		householdID, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FoodLogEntry
	for rows.Next() {
		var e domain.FoodLogEntry
		var created int64
		if err := rows.Scan(&e.ID, &e.HouseholdID, &e.UserID, &e.Date, &e.ProductID, &e.Quantity, &e.Unit, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = fromTS(created)
		out = append(out, e)
	}
	return out, rows.Err()
}
