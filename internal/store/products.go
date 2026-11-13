package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

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
	return productUsageTx(ctx, s, nil, householdID, productID)
}

func productUsageTx(ctx context.Context, s *Store, tx *sql.Tx, householdID, productID string) (ProductUsage, error) {
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
		var row *sql.Row
		if tx != nil {
			row = tx.QueryRowContext(ctx, s.db.Rebind(q.sql), householdID, productID)
		} else {
			row = s.db.QueryRowContext(ctx, s.db.Rebind(q.sql), householdID, productID)
		}
		if err := row.Scan(q.dst); err != nil {
			return ProductUsage{}, err
		}
	}
	return u, nil
}

// MaxAliases is the most aliases one product may have.
const MaxAliases = 100

// MergeResult reports what MergeProducts moved: Repointed counts the references the absorbed
// products had, SkippedAliases lists names that could not become aliases of the kept product
// (owned by a third product, or past MaxAliases).
type MergeResult struct {
	Repointed      ProductUsage
	SkippedAliases []string
}

// MergeProducts folds the absorbed products into keepID in one transaction: ingredients, food log,
// shopping items, pantry movements and pantry stock move to the kept product (stock of the same
// unit is summed), and the absorbed products' names and aliases become aliases of the kept one.
// Unknown or foreign products, and keepID among absorbIDs, are ErrNotFound / ErrConflict respectively.
func (s *Store) MergeProducts(ctx context.Context, householdID, keepID string, absorbIDs []string) (MergeResult, error) {
	var res MergeResult
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		res = MergeResult{}
		var keepName string
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT normalized_name FROM product WHERE id = ? AND household_id = ?`), keepID, householdID).Scan(&keepName); err != nil {
			return mapErr(err)
		}
		var absorbed []string
		seen := map[string]bool{keepID: true}
		for _, id := range absorbIDs {
			if id == keepID {
				return ErrConflict
			}
			if !seen[id] {
				seen[id] = true
				absorbed = append(absorbed, id)
			}
		}
		names := make(map[string]string, len(absorbed))
		for _, id := range absorbed {
			var n string
			if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT normalized_name FROM product WHERE id = ? AND household_id = ?`), id, householdID).Scan(&n); err != nil {
				return mapErr(err)
			}
			names[id] = n
			u, err := productUsageTx(ctx, s, tx, householdID, id)
			if err != nil {
				return err
			}
			res.Repointed.Recipes += u.Recipes
			res.Repointed.Pantry += u.Pantry
			res.Repointed.FoodLog += u.FoodLog
			res.Repointed.Shopping += u.Shopping
		}
		var aliasCount int
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM product_alias WHERE household_id = ? AND product_id = ?`), householdID, keepID).Scan(&aliasCount); err != nil {
			return err
		}
		for _, id := range absorbed {
			for _, q := range []string{
				`UPDATE ingredient SET product_id = ? WHERE product_id = ? AND recipe_id IN (SELECT id FROM recipe WHERE household_id = ?)`,
				`UPDATE food_log SET product_id = ? WHERE product_id = ? AND household_id = ?`,
				`UPDATE shopping_item SET product_id = ? WHERE product_id = ? AND household_id = ?`,
				`UPDATE pantry_movement SET product_id = ? WHERE product_id = ? AND household_id = ?`,
			} {
				if err := s.exec(ctx, tx, q, keepID, id, householdID); err != nil {
					return err
				}
			}
			if err := s.mergePantry(ctx, tx, householdID, keepID, id); err != nil {
				return err
			}
			aliases, err := s.txStrings(ctx, tx, `SELECT alias FROM product_alias WHERE household_id = ? AND product_id = ? ORDER BY alias`, householdID, id)
			if err != nil {
				return err
			}
			for _, a := range aliases {
				switch {
				case a == keepName:
					// redundant: the kept product already answers to this name
				case aliasCount >= MaxAliases:
					res.SkippedAliases = append(res.SkippedAliases, a)
				default:
					if err := s.exec(ctx, tx, `UPDATE product_alias SET product_id = ? WHERE household_id = ? AND alias = ?`, keepID, householdID, a); err != nil {
						return err
					}
					aliasCount++
				}
			}
			if err := s.exec(ctx, tx, `DELETE FROM product_alias WHERE household_id = ? AND product_id = ?`, householdID, id); err != nil {
				return err
			}
		}
		for _, id := range absorbed {
			name := names[id]
			if name == keepName {
				continue
			}
			var owner string
			err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT product_id FROM product_alias WHERE household_id = ? AND alias = ?`), householdID, name).Scan(&owner)
			switch {
			case err == nil && owner == keepID:
				continue
			case err == nil:
				res.SkippedAliases = append(res.SkippedAliases, name)
				continue
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
			if aliasCount >= MaxAliases {
				res.SkippedAliases = append(res.SkippedAliases, name)
				continue
			}
			if err := s.exec(ctx, tx, `INSERT INTO product_alias (household_id, product_id, alias) VALUES (?, ?, ?)`, householdID, keepID, name); err != nil {
				return err
			}
			aliasCount++
		}
		// Fill what the kept product lacks (macros, piece weight, density) from the absorbed ones in
		// order; a value the kept product already has is never overwritten.
		for _, id := range absorbed {
			if err := s.exec(ctx, tx, `UPDATE product SET
				kcal = COALESCE(kcal, (SELECT kcal FROM product WHERE id = ? AND household_id = ?)),
				protein = COALESCE(protein, (SELECT protein FROM product WHERE id = ? AND household_id = ?)),
				carbs = COALESCE(carbs, (SELECT carbs FROM product WHERE id = ? AND household_id = ?)),
				fat = COALESCE(fat, (SELECT fat FROM product WHERE id = ? AND household_id = ?)),
				piece_grams = COALESCE(piece_grams, (SELECT piece_grams FROM product WHERE id = ? AND household_id = ?)),
				density = COALESCE(density, (SELECT density FROM product WHERE id = ? AND household_id = ?))
				WHERE id = ? AND household_id = ?`,
				id, householdID, id, householdID, id, householdID, id, householdID, id, householdID, id, householdID, keepID, householdID); err != nil {
				return err
			}
		}
		for _, id := range absorbed {
			if err := s.exec(ctx, tx, `DELETE FROM product WHERE id = ? AND household_id = ?`, id, householdID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MergeResult{}, err
	}
	return res, nil
}

type pantryRow struct {
	id, unit, expires string
	qty               sql.NullFloat64
	staple            bool
}

func (s *Store) pantryRows(ctx context.Context, tx *sql.Tx, householdID, productID string) ([]pantryRow, error) {
	rows, err := tx.QueryContext(ctx, s.db.Rebind(`SELECT id, unit, quantity, expires_on, staple FROM pantry_item WHERE household_id = ? AND product_id = ? ORDER BY id`), householdID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pantryRow
	for rows.Next() {
		var r pantryRow
		var staple int
		if err := rows.Scan(&r.id, &r.unit, &r.qty, &r.expires, &staple); err != nil {
			return nil, err
		}
		r.staple = staple != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// mergePantry moves the absorbed product's stock to keepID: a row in a unit the kept product
// already has is folded into it (sum, earliest expiry, staple wins), any other row is re-pointed.
func (s *Store) mergePantry(ctx context.Context, tx *sql.Tx, householdID, keepID, absorbID string) error {
	theirs, err := s.pantryRows(ctx, tx, householdID, absorbID)
	if err != nil {
		return err
	}
	for _, b := range theirs {
		mine, err := s.pantryRows(ctx, tx, householdID, keepID)
		if err != nil {
			return err
		}
		var a *pantryRow
		for i := range mine {
			if mine[i].unit == b.unit {
				a = &mine[i]
			}
		}
		if a == nil {
			if err := s.exec(ctx, tx, `UPDATE pantry_item SET product_id = ? WHERE id = ?`, keepID, b.id); err != nil {
				return err
			}
			continue
		}
		staple := a.staple || b.staple || !a.qty.Valid || !b.qty.Valid
		var qty any
		if !staple {
			qty = a.qty.Float64 + b.qty.Float64
		}
		expires := a.expires
		if b.expires != "" && (expires == "" || b.expires < expires) {
			expires = b.expires
		}
		if err := s.exec(ctx, tx, `UPDATE pantry_item SET quantity = ?, expires_on = ?, staple = ?, updated_at = ? WHERE id = ?`,
			qty, expires, b2i(staple), ts(time.Now()), a.id); err != nil {
			return err
		}
		if err := s.exec(ctx, tx, `DELETE FROM pantry_item WHERE id = ?`, b.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) txStrings(ctx context.Context, tx *sql.Tx, q string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, s.db.Rebind(q), args...)
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
