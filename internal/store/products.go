package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"mealplan/internal/domain"
)

func nf(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func pf(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

func (s *Store) insertProduct(ctx context.Context, tx *sql.Tx, p domain.Product) error {
	if err := s.exec(ctx, tx, `INSERT INTO product (id, household_id, name, normalized_name, kcal, protein, carbs, fat, piece_grams, density, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.HouseholdID, p.Name, p.NormalizedName, nf(p.Per100g.Kcal), nf(p.Per100g.Protein), nf(p.Per100g.Carbs), nf(p.Per100g.Fat),
		nf(p.PieceGrams), nf(p.DensityGPerML), ts(p.CreatedAt), ts(p.UpdatedAt)); err != nil {
		return err
	}
	return s.insertAliases(ctx, tx, p)
}

func (s *Store) insertAliases(ctx context.Context, tx *sql.Tx, p domain.Product) error {
	for _, a := range p.Aliases {
		if err := s.exec(ctx, tx, `INSERT INTO product_alias (household_id, product_id, alias) VALUES (?, ?, ?)`, p.HouseholdID, p.ID, a); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateProduct(ctx context.Context, p domain.Product) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return s.insertProduct(ctx, tx, p) })
}

func (s *Store) UpdateProduct(ctx context.Context, p domain.Product) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, s.db.Rebind(`UPDATE product SET name = ?, normalized_name = ?, kcal = ?, protein = ?, carbs = ?, fat = ?, piece_grams = ?, density = ?, updated_at = ? WHERE id = ? AND household_id = ?`),
			p.Name, p.NormalizedName, nf(p.Per100g.Kcal), nf(p.Per100g.Protein), nf(p.Per100g.Carbs), nf(p.Per100g.Fat),
			nf(p.PieceGrams), nf(p.DensityGPerML), ts(p.UpdatedAt), p.ID, p.HouseholdID)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if err := s.exec(ctx, tx, `DELETE FROM product_alias WHERE product_id = ? AND household_id = ?`, p.ID, p.HouseholdID); err != nil {
			return err
		}
		return s.insertAliases(ctx, tx, p)
	})
}

func (s *Store) DeleteProduct(ctx context.Context, householdID, id string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM product WHERE id = ? AND household_id = ?`), id, householdID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM food_log WHERE product_id = ?`), id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrInUse
		}
		return s.exec(ctx, tx, `DELETE FROM product WHERE id = ? AND household_id = ?`, id, householdID)
	})
}

const productCols = `id, household_id, name, normalized_name, kcal, protein, carbs, fat, piece_grams, density, created_at, updated_at`

func scanProduct(r scanner) (domain.Product, error) {
	var p domain.Product
	var k, pr, c, f, pg, d sql.NullFloat64
	var created, updated int64
	if err := r.Scan(&p.ID, &p.HouseholdID, &p.Name, &p.NormalizedName, &k, &pr, &c, &f, &pg, &d, &created, &updated); err != nil {
		return domain.Product{}, err
	}
	p.Per100g = domain.Macros{Kcal: pf(k), Protein: pf(pr), Carbs: pf(c), Fat: pf(f)}
	p.PieceGrams, p.DensityGPerML = pf(pg), pf(d)
	p.CreatedAt, p.UpdatedAt = fromTS(created), fromTS(updated)
	return p, nil
}

func (s *Store) ProductByID(ctx context.Context, householdID, id string) (domain.Product, error) {
	p, err := scanProduct(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+productCols+` FROM product WHERE id = ? AND household_id = ?`), id, householdID))
	if err != nil {
		return domain.Product{}, mapErr(err)
	}
	if p.Aliases, err = s.stringCol(ctx, `SELECT alias FROM product_alias WHERE product_id = ? ORDER BY alias`, id); err != nil {
		return domain.Product{}, err
	}
	return p, nil
}

// ListProducts returns the household's products with aliases, ordered by name.
func (s *Store) ListProducts(ctx context.Context, householdID string) ([]domain.Product, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT `+productCols+` FROM product WHERE household_id = ? ORDER BY normalized_name, id`), householdID)
	if err != nil {
		return nil, err
	}
	var out []domain.Product
	idx := map[string]int{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		idx[p.ID] = len(out)
		out = append(out, p)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	arows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT product_id, alias FROM product_alias WHERE household_id = ? ORDER BY alias`), householdID)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	for arows.Next() {
		var id, alias string
		if err := arows.Scan(&id, &alias); err != nil {
			return nil, err
		}
		if i, ok := idx[id]; ok {
			out[i].Aliases = append(out[i].Aliases, alias)
		}
	}
	return out, arows.Err()
}

// SeedProducts inserts ps once per household; it reports whether it seeded.
func (s *Store) SeedProducts(ctx context.Context, householdID string, ps []domain.Product) (bool, error) {
	seeded := false
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, s.db.Rebind(`UPDATE household SET products_seeded = 1 WHERE id = ? AND products_seeded = 0`), householdID)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		for _, p := range ps {
			p.HouseholdID = householdID
			if err := s.insertProduct(ctx, tx, p); err != nil {
				return err
			}
		}
		seeded = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return seeded, nil
}

// AssignIngredients links the ingredients named normalizedName in the given recipes of the household
// to the product and, when alias is non-empty and free, remembers it as an alias of the product.
// If another product already owns the alias (as its name or alias), that product's name is returned
// in aliasOwner and the alias is not added. An unknown product is ErrNotFound.
func (s *Store) AssignIngredients(ctx context.Context, householdID, productID, normalizedName string, recipeIDs []string, alias string) (updated int, aliasOwner string, err error) {
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		updated, aliasOwner = 0, ""
		var ownName string
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT normalized_name FROM product WHERE id = ? AND household_id = ?`), productID, householdID).Scan(&ownName); err != nil {
			return mapErr(err)
		}
		if len(recipeIDs) > 0 {
			ph := strings.TrimSuffix(strings.Repeat("?, ", len(recipeIDs)), ", ")
			args := []any{productID, normalizedName, householdID}
			for _, id := range recipeIDs {
				args = append(args, id)
			}
			res, err := tx.ExecContext(ctx, s.db.Rebind(`UPDATE ingredient SET product_id = ? WHERE normalized_name = ?
				AND recipe_id IN (SELECT id FROM recipe WHERE household_id = ? AND id IN (`+ph+`))`), args...)
			if err != nil {
				return mapErr(err)
			}
			n, _ := res.RowsAffected()
			updated = int(n)
		}
		if alias == "" || alias == ownName {
			return nil
		}
		var ownerID string
		err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT product_id FROM product_alias WHERE household_id = ? AND alias = ?`), householdID, alias).Scan(&ownerID)
		switch {
		case err == nil:
			if ownerID == productID {
				return nil
			}
		case errors.Is(err, sql.ErrNoRows):
			ownerID = ""
		default:
			return err
		}
		if ownerID == "" {
			err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT id FROM product WHERE household_id = ? AND normalized_name = ?`), householdID, alias).Scan(&ownerID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if ownerID != "" {
			return tx.QueryRowContext(ctx, s.db.Rebind(`SELECT name FROM product WHERE id = ?`), ownerID).Scan(&aliasOwner)
		}
		return s.exec(ctx, tx, `INSERT INTO product_alias (household_id, product_id, alias) VALUES (?, ?, ?)`, householdID, productID, alias)
	})
	if err != nil {
		return 0, "", err
	}
	return updated, aliasOwner, nil
}

// ProductUsage counts where a product is referenced; Recipes is the number of distinct recipes.
type ProductUsage struct{ Recipes, Pantry, FoodLog, Shopping int }

func (s *Store) ProductUsage(ctx context.Context, householdID, productID string) (ProductUsage, error) {
	var u ProductUsage
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&u.Recipes, `SELECT COUNT(DISTINCT i.recipe_id) FROM ingredient i JOIN recipe r ON r.id = i.recipe_id WHERE r.household_id = ? AND i.product_id = ?`},
		{&u.Pantry, `SELECT COUNT(*) FROM pantry_item WHERE household_id = ? AND product_id = ?`},
		{&u.FoodLog, `SELECT COUNT(*) FROM food_log WHERE household_id = ? AND product_id = ?`},
		{&u.Shopping, `SELECT COUNT(*) FROM shopping_item WHERE household_id = ? AND product_id = ?`},
	} {
		if err := s.db.QueryRowContext(ctx, s.db.Rebind(q.sql), householdID, productID).Scan(q.dst); err != nil {
			return ProductUsage{}, err
		}
	}
	return u, nil
}
