package store

import (
	"context"
	"database/sql"

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
