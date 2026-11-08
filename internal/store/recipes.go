package store

import (
	"context"
	"database/sql"

	"mealplan/internal/domain"
)

func (s *Store) CreateRecipe(ctx context.Context, r domain.Recipe) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.exec(ctx, tx, `INSERT INTO recipe (id, household_id, title, base_servings, source_url, kcal, protein, carbs, fat, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.HouseholdID, r.Title, r.BaseServings, r.SourceURL, nf(r.Nutrition.Kcal), nf(r.Nutrition.Protein), nf(r.Nutrition.Carbs), nf(r.Nutrition.Fat), ts(r.CreatedAt), ts(r.UpdatedAt)); err != nil {
			return err
		}
		return s.insertRecipeChildren(ctx, tx, r)
	})
}

func (s *Store) UpdateRecipe(ctx context.Context, r domain.Recipe) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, s.db.Rebind(`UPDATE recipe SET title = ?, base_servings = ?, source_url = ?, kcal = ?, protein = ?, carbs = ?, fat = ?, updated_at = ? WHERE id = ? AND household_id = ?`),
			r.Title, r.BaseServings, r.SourceURL, nf(r.Nutrition.Kcal), nf(r.Nutrition.Protein), nf(r.Nutrition.Carbs), nf(r.Nutrition.Fat), ts(r.UpdatedAt), r.ID, r.HouseholdID)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		for _, tbl := range []string{"ingredient", "step", "recipe_tag"} {
			if err := s.exec(ctx, tx, `DELETE FROM `+tbl+` WHERE recipe_id = ?`, r.ID); err != nil {
				return err
			}
		}
		return s.insertRecipeChildren(ctx, tx, r)
	})
}

func (s *Store) insertRecipeChildren(ctx context.Context, tx *sql.Tx, r domain.Recipe) error {
	for i, ing := range r.Ingredients {
		var q any
		if ing.Quantity != nil {
			q = *ing.Quantity
		}
		var pid any
		if ing.ProductID != "" {
			pid = ing.ProductID
		}
		if err := s.exec(ctx, tx, `INSERT INTO ingredient (recipe_id, position, name, normalized_name, quantity, unit, note, product_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, i, ing.Name, ing.NormalizedName, q, ing.Unit, ing.Note, pid); err != nil {
			return err
		}
	}
	for i, text := range r.Steps {
		if err := s.exec(ctx, tx, `INSERT INTO step (recipe_id, position, text) VALUES (?, ?, ?)`, r.ID, i, text); err != nil {
			return err
		}
	}
	for _, tag := range r.Tags {
		if err := s.exec(ctx, tx, `INSERT INTO recipe_tag (recipe_id, tag) VALUES (?, ?)`, r.ID, tag); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecipeByID(ctx context.Context, householdID, id string) (domain.Recipe, error) {
	var r domain.Recipe
	var created, updated int64
	var nk, np, nc, nfat sql.NullFloat64
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT id, household_id, title, base_servings, source_url, kcal, protein, carbs, fat, created_at, updated_at FROM recipe WHERE id = ? AND household_id = ?`),
		id, householdID).Scan(&r.ID, &r.HouseholdID, &r.Title, &r.BaseServings, &r.SourceURL, &nk, &np, &nc, &nfat, &created, &updated)
	if err != nil {
		return domain.Recipe{}, mapErr(err)
	}
	r.CreatedAt, r.UpdatedAt = fromTS(created), fromTS(updated)
	r.Nutrition = domain.Macros{Kcal: pf(nk), Protein: pf(np), Carbs: pf(nc), Fat: pf(nfat)}

	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT name, normalized_name, quantity, unit, note, product_id FROM ingredient WHERE recipe_id = ? ORDER BY position`), id)
	if err != nil {
		return domain.Recipe{}, err
	}
	for rows.Next() {
		var ing domain.Ingredient
		var q sql.NullFloat64
		var pid sql.NullString
		if err := rows.Scan(&ing.Name, &ing.NormalizedName, &q, &ing.Unit, &ing.Note, &pid); err != nil {
			_ = rows.Close()
			return domain.Recipe{}, err
		}
		ing.ProductID = pid.String
		if q.Valid {
			v := q.Float64
			ing.Quantity = &v
		}
		r.Ingredients = append(r.Ingredients, ing)
	}
	if err := rows.Close(); err != nil {
		return domain.Recipe{}, err
	}

	if r.Steps, err = s.stringCol(ctx, `SELECT text FROM step WHERE recipe_id = ? ORDER BY position`, id); err != nil {
		return domain.Recipe{}, err
	}
	if r.Tags, err = s.stringCol(ctx, `SELECT tag FROM recipe_tag WHERE recipe_id = ? ORDER BY tag`, id); err != nil {
		return domain.Recipe{}, err
	}
	return r, nil
}

func (s *Store) stringCol(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListRecipes returns summaries (title, servings, source, tags) ordered by title.
func (s *Store) ListRecipes(ctx context.Context, householdID string) ([]domain.Recipe, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT id, household_id, title, base_servings, source_url, created_at, updated_at FROM recipe WHERE household_id = ? ORDER BY title, id`), householdID)
	if err != nil {
		return nil, err
	}
	var out []domain.Recipe
	idx := map[string]int{}
	for rows.Next() {
		var r domain.Recipe
		var created, updated int64
		if err := rows.Scan(&r.ID, &r.HouseholdID, &r.Title, &r.BaseServings, &r.SourceURL, &created, &updated); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.CreatedAt, r.UpdatedAt = fromTS(created), fromTS(updated)
		idx[r.ID] = len(out)
		out = append(out, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	trows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT t.recipe_id, t.tag FROM recipe_tag t JOIN recipe r ON r.id = t.recipe_id WHERE r.household_id = ? ORDER BY t.tag`), householdID)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var id, tag string
		if err := trows.Scan(&id, &tag); err != nil {
			return nil, err
		}
		if i, ok := idx[id]; ok {
			out[i].Tags = append(out[i].Tags, tag)
		}
	}
	return out, trows.Err()
}

func (s *Store) DeleteRecipe(ctx context.Context, householdID, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM recipe WHERE id = ? AND household_id = ?`), id, householdID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM plan_entry WHERE recipe_id = ? AND household_id = ?`), id, householdID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrInUse
		}
		return s.exec(ctx, tx, `DELETE FROM recipe WHERE id = ? AND household_id = ?`, id, householdID)
	})
}
