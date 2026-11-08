package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"mealplan/internal/domain"
)

func newPantryID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

const pantryCols = `i.id, i.household_id, i.product_id, p.name, i.quantity, i.unit, i.expires_on, i.staple, i.updated_at`
const pantryFrom = ` FROM pantry_item i JOIN product p ON p.id = i.product_id `

func scanPantry(r scanner) (domain.PantryItem, error) {
	var it domain.PantryItem
	var q sql.NullFloat64
	var staple int
	var updated int64
	if err := r.Scan(&it.ID, &it.HouseholdID, &it.ProductID, &it.Name, &q, &it.Unit, &it.ExpiresOn, &staple, &updated); err != nil {
		return domain.PantryItem{}, mapErr(err)
	}
	it.Quantity, it.Staple, it.UpdatedAt = pf(q), staple != 0, fromTS(updated)
	return it, nil
}

// PantryItems lists the household's pantry ordered by product name.
func (s *Store) PantryItems(ctx context.Context, householdID string) ([]domain.PantryItem, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT `+pantryCols+pantryFrom+`WHERE i.household_id = ? ORDER BY p.normalized_name, i.unit, i.id`), householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PantryItem
	for rows.Next() {
		it, err := scanPantry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) PantryItemByID(ctx context.Context, householdID, id string) (domain.PantryItem, error) {
	return scanPantry(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+pantryCols+pantryFrom+`WHERE i.id = ? AND i.household_id = ?`), id, householdID))
}

// stockRow is a pantry row looked up by (product, unit) inside a transaction.
type stockRow struct {
	id  string
	qty sql.NullFloat64
}

func (s *Store) findStock(ctx context.Context, tx *sql.Tx, householdID, productID, unit string) (stockRow, bool, error) {
	var r stockRow
	q := `SELECT id, quantity FROM pantry_item WHERE household_id = ? AND product_id = ? AND unit = ?`
	if s.db.Driver == "postgres" {
		// Lock the row so concurrent movements serialize and clamp against the current value.
		// SQLite needs no lock: the pool has a single connection, so transactions are serialized.
		q += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, s.db.Rebind(q),
		householdID, productID, unit).Scan(&r.id, &r.qty)
	if err == sql.ErrNoRows {
		return r, false, nil
	}
	return r, err == nil, err
}

// requireProduct makes sure the product belongs to the household (FKs alone do not check that).
func (s *Store) requireProduct(ctx context.Context, tx *sql.Tx, householdID, productID string) error {
	var n int
	if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM product WHERE id = ? AND household_id = ?`), productID, householdID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) insertStock(ctx context.Context, tx *sql.Tx, householdID, productID, unit string, qty float64) (string, error) {
	if err := s.requireProduct(ctx, tx, householdID, productID); err != nil {
		return "", err
	}
	id := newPantryID()
	err := s.exec(ctx, tx, `INSERT INTO pantry_item (id, household_id, product_id, quantity, unit, expires_on, staple, updated_at) VALUES (?, ?, ?, ?, ?, '', 0, ?)`,
		id, householdID, productID, qty, unit, ts(time.Now()))
	return id, err
}

// AddToPantry upserts by (product, unit): it adds to the existing quantity or creates the row.
// A staple row (quantity NULL) stays a staple.
func (s *Store) AddToPantry(ctx context.Context, householdID, productID, unit string, amount float64) (domain.PantryItem, error) {
	var id string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.requireProduct(ctx, tx, householdID, productID); err != nil {
			return err
		}
		// Atomic upsert; a staple row (NULL quantity) stays a staple.
		return tx.QueryRowContext(ctx, s.db.Rebind(`INSERT INTO pantry_item (id, household_id, product_id, quantity, unit, expires_on, staple, updated_at)
VALUES (?, ?, ?, ?, ?, '', 0, ?)
ON CONFLICT (household_id, product_id, unit) DO UPDATE SET
  quantity = CASE WHEN pantry_item.quantity IS NULL THEN NULL ELSE pantry_item.quantity + excluded.quantity END,
  updated_at = excluded.updated_at
RETURNING id`), newPantryID(), householdID, productID, amount, unit, ts(time.Now())).Scan(&id)
	})
	if err != nil {
		return domain.PantryItem{}, err
	}
	return s.PantryItemByID(ctx, householdID, id)
}

func (s *Store) SavePantryItem(ctx context.Context, it domain.PantryItem) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE pantry_item SET quantity = ?, unit = ?, expires_on = ?, staple = ?, updated_at = ? WHERE id = ? AND household_id = ?`),
		nf(it.Quantity), it.Unit, it.ExpiresOn, b2i(it.Staple), ts(time.Now()), it.ID, it.HouseholdID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePantryItem(ctx context.Context, householdID, id string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM pantry_item WHERE id = ? AND household_id = ?`), id, householdID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ApplyMovements applies all movements in one transaction and records each one with the amount
// ACTUALLY applied, so undoing a movement (applying its negation) restores exactly what was taken.
//
//   - Negative amounts never take the quantity below zero: only what is in stock is taken and
//     recorded (300 g in stock, -500 requested: -300 is applied and recorded).
//   - A negative movement for a (product, unit) with no pantry row takes nothing and records nothing.
//   - A positive movement for a (product, unit) with no pantry row creates the row.
//   - Staple rows (quantity NULL) are never changed and nothing is recorded for them.
//   - Movements that end up applying 0 are not recorded.
func (s *Store) ApplyMovements(ctx context.Context, householdID string, ms []domain.PantryMovement) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return s.applyMovements(ctx, tx, householdID, ms) })
}

// lockEntry locks the household's plan entry row for the transaction (ErrNotFound if absent).
// SQLite needs no lock: the pool has a single connection, so transactions are serialized.
func (s *Store) lockEntry(ctx context.Context, tx *sql.Tx, householdID, entryID string) error {
	q := `SELECT id FROM plan_entry WHERE id = ? AND household_id = ?`
	if s.db.Driver == "postgres" {
		q += ` FOR UPDATE`
	}
	var id string
	if err := tx.QueryRowContext(ctx, s.db.Rebind(q), entryID, householdID).Scan(&id); err != nil {
		return mapErr(err)
	}
	return nil
}

func (s *Store) movementsTx(ctx context.Context, tx *sql.Tx, householdID, entryID string) (int, []domain.PantryMovement, error) {
	rows, err := tx.QueryContext(ctx, s.db.Rebind(`SELECT entry_id, product_id, unit, amount FROM pantry_movement WHERE household_id = ? AND entry_id = ? ORDER BY product_id, unit, amount`), householdID, entryID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var out []domain.PantryMovement
	for rows.Next() {
		var m domain.PantryMovement
		if err := rows.Scan(&m.EntryID, &m.ProductID, &m.Unit, &m.Amount); err != nil {
			return 0, nil, err
		}
		out = append(out, m)
	}
	return len(out), out, rows.Err()
}

// ConsumeForEntry applies ms in one transaction unless the entry already has recorded movements.
// The entry row is locked first, so concurrent calls for one entry deduct exactly once.
func (s *Store) ConsumeForEntry(ctx context.Context, householdID, entryID string, ms []domain.PantryMovement) (bool, error) {
	applied := false
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.lockEntry(ctx, tx, householdID, entryID); err != nil {
			return err
		}
		n, _, err := s.movementsTx(ctx, tx, householdID, entryID)
		if err != nil || n > 0 {
			return err
		}
		for _, m := range ms {
			if m.EntryID != entryID {
				return ErrNotFound
			}
		}
		applied = true
		return s.applyMovements(ctx, tx, householdID, ms)
	})
	return applied && err == nil, err
}

// RestoreForEntry gives back the recorded (actually applied) movements of the entry with the opposite
// sign and forgets them, all in one transaction under the entry lock. No movements means no-op.
func (s *Store) RestoreForEntry(ctx context.Context, householdID, entryID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.lockEntry(ctx, tx, householdID, entryID); err != nil {
			return err
		}
		n, ms, err := s.movementsTx(ctx, tx, householdID, entryID)
		if err != nil || n == 0 {
			return err
		}
		for i := range ms {
			ms[i].Amount = -ms[i].Amount
		}
		if err := s.applyMovements(ctx, tx, householdID, ms); err != nil {
			return err
		}
		return s.exec(ctx, tx, `DELETE FROM pantry_movement WHERE household_id = ? AND entry_id = ?`, householdID, entryID)
	})
}

func (s *Store) applyMovements(ctx context.Context, tx *sql.Tx, householdID string, ms []domain.PantryMovement) error {
	{
		for _, m := range ms {
			var n int
			if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM plan_entry WHERE id = ? AND household_id = ?`), m.EntryID, householdID).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return ErrNotFound
			}
			r, ok, err := s.findStock(ctx, tx, householdID, m.ProductID, m.Unit)
			if err != nil {
				return err
			}
			applied := m.Amount
			switch {
			case !ok:
				if m.Amount <= 0 {
					continue
				}
				if _, err := s.insertStock(ctx, tx, householdID, m.ProductID, m.Unit, m.Amount); err != nil {
					return err
				}
			case !r.qty.Valid:
				continue
			default:
				next := r.qty.Float64 + m.Amount
				if next < 0 {
					next, applied = 0, -r.qty.Float64
				}
				if applied == 0 {
					continue
				}
				if err := s.exec(ctx, tx, `UPDATE pantry_item SET quantity = ?, updated_at = ? WHERE id = ? AND household_id = ?`,
					next, ts(time.Now()), r.id, householdID); err != nil {
					return err
				}
			}
			if err := s.exec(ctx, tx, `INSERT INTO pantry_movement (household_id, entry_id, product_id, unit, amount) VALUES (?, ?, ?, ?, ?)`,
				householdID, m.EntryID, m.ProductID, m.Unit, applied); err != nil {
				return err
			}
		}
		return nil
	}
}

func (s *Store) MovementsForEntry(ctx context.Context, householdID, entryID string) ([]domain.PantryMovement, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT entry_id, product_id, unit, amount FROM pantry_movement WHERE household_id = ? AND entry_id = ? ORDER BY product_id, unit, amount`), householdID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PantryMovement
	for rows.Next() {
		var m domain.PantryMovement
		if err := rows.Scan(&m.EntryID, &m.ProductID, &m.Unit, &m.Amount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) DeleteMovementsForEntry(ctx context.Context, householdID, entryID string) error {
	return s.exec(ctx, s.db, `DELETE FROM pantry_movement WHERE household_id = ? AND entry_id = ?`, householdID, entryID)
}

// PantryReviewedAt returns when the household last reviewed its pantry, nil if never.
func (s *Store) PantryReviewedAt(ctx context.Context, householdID string) (*time.Time, error) {
	var n sql.NullInt64
	if err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT pantry_reviewed_at FROM household WHERE id = ?`), householdID).Scan(&n); err != nil {
		return nil, mapErr(err)
	}
	if !n.Valid {
		return nil, nil
	}
	t := fromTS(n.Int64)
	return &t, nil
}

func (s *Store) MarkPantryReviewed(ctx context.Context, householdID string, at time.Time) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE household SET pantry_reviewed_at = ? WHERE id = ?`), ts(at), householdID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetShoppingItemsMoved marks the household's shopping items as moved to the pantry.
func (s *Store) SetShoppingItemsMoved(ctx context.Context, householdID string, ids []string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			if err := s.exec(ctx, tx, `UPDATE shopping_item SET moved = 1 WHERE id = ? AND household_id = ?`, id, householdID); err != nil {
				return err
			}
		}
		return nil
	})
}
