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

// AssistantSettings returns the household's OpenRouter key and model ("" when not set).
func (s *Store) AssistantSettings(ctx context.Context, householdID string) (key, model string, err error) {
	if err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT openrouter_key, openrouter_model FROM household WHERE id = ?`), householdID).Scan(&key, &model); err != nil {
		return "", "", mapErr(err)
	}
	return key, model, nil
}

// SetAssistantSettings changes the household's OpenRouter key and model; an unknown household is ErrNotFound.
func (s *Store) SetAssistantSettings(ctx context.Context, householdID, key, model string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE household SET openrouter_key = ?, openrouter_model = ? WHERE id = ?`), key, model, householdID)
	if err != nil {
		return mapErr(err)
	}
	if c, _ := res.RowsAffected(); c == 0 {
		return ErrNotFound
	}
	return nil
}
