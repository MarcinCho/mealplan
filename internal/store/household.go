package store

import "context"

// PortionsPerMeal is the household's default number of portions per meal (2 unless changed).
func (s *Store) PortionsPerMeal(ctx context.Context, householdID string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT portions_per_meal FROM household WHERE id = ?`), householdID).Scan(&n); err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

// SetPortionsPerMeal changes the household setting; an unknown household is ErrNotFound.
func (s *Store) SetPortionsPerMeal(ctx context.Context, householdID string, n int) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE household SET portions_per_meal = ? WHERE id = ?`), n, householdID)
	if err != nil {
		return mapErr(err)
	}
	if c, _ := res.RowsAffected(); c == 0 {
		return ErrNotFound
	}
	return nil
}
