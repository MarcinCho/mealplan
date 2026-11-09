package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func seedPantry(t *testing.T, s *store.Store) {
	t.Helper()
	seedTwoHouseholds(t, s)
	ctx := context.Background()
	for _, p := range []domain.Product{prod("p1", "hA", "mąka"), prod("p2", "hA", "jajko")} {
		if err := s.CreateProduct(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAddToPantryMergesSameProductAndUnit(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	if _, err := s.AddToPantry(ctx, "hA", "p1", "g", 500); err != nil {
		t.Fatal(err)
	}
	it, err := s.AddToPantry(ctx, "hA", "p1", "g", 500)
	if err != nil || it.Quantity == nil || *it.Quantity != 1000 || it.Name != "mąka" {
		t.Fatalf("merge: %v %+v", err, it)
	}
	if _, err := s.AddToPantry(ctx, "hA", "p1", "szt", 2); err != nil {
		t.Fatal(err)
	}
	items, err := s.PantryItems(ctx, "hA")
	if err != nil || len(items) != 2 {
		t.Fatalf("items: %v %+v", err, items)
	}
	// Foreign product is rejected.
	if _, err := s.AddToPantry(ctx, "hB", "p1", "g", 1); err == nil {
		t.Fatal("expected error for foreign product")
	}
}

func TestSavePantryItemAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	it, _ := s.AddToPantry(ctx, "hA", "p1", "g", 100)
	it.Quantity, it.ExpiresOn, it.Staple = nil, "2026-12-01", true
	if err := s.SavePantryItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	got, err := s.PantryItemByID(ctx, "hA", it.ID)
	if err != nil || got.Quantity != nil || got.ExpiresOn != "2026-12-01" || !got.Staple {
		t.Fatalf("saved: %v %+v", err, got)
	}
	it.HouseholdID = "hB"
	if err := s.SavePantryItem(ctx, it); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("save foreign: %v", err)
	}
	if err := s.DeletePantryItem(ctx, "hB", it.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete foreign: %v", err)
	}
	if err := s.DeletePantryItem(ctx, "hA", it.ID); err != nil {
		t.Fatal(err)
	}
}

func TestApplyMovementsNeverBelowZero(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	if err := s.InsertPlanEntry(ctx, entry("e1", "hA", "2026-11-02", domain.SlotDinner)); err != nil {
		t.Fatal(err)
	}
	it, _ := s.AddToPantry(ctx, "hA", "p1", "g", 300)
	err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{{EntryID: "e1", ProductID: "p1", Unit: "g", Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.PantryItemByID(ctx, "hA", it.ID)
	if got.Quantity == nil || *got.Quantity != 0 {
		t.Fatalf("quantity: %+v", got)
	}
	ms, err := s.MovementsForEntry(ctx, "hA", "e1")
	if err != nil || len(ms) != 1 || ms[0].Amount != -300 {
		t.Fatalf("movements: %v %+v", err, ms)
	}
	// Undo returns exactly what was taken.
	undo := []domain.PantryMovement{{EntryID: "e1", ProductID: "p1", Unit: "g", Amount: -ms[0].Amount}}
	if err := s.ApplyMovements(ctx, "hA", undo); err != nil {
		t.Fatal(err)
	}
	got, _ = s.PantryItemByID(ctx, "hA", it.ID)
	if *got.Quantity != 300 {
		t.Fatalf("after undo: %+v", *got.Quantity)
	}
}

func TestApplyMovementsMissingRowAndStaple(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_ = s.InsertPlanEntry(ctx, entry("e1", "hA", "2026-11-02", domain.SlotDinner))
	// Negative with no row: nothing taken, nothing recorded, no row created.
	// Positive with no row: row created and movement recorded.
	err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{
		{EntryID: "e1", ProductID: "p1", Unit: "g", Amount: -100},
		{EntryID: "e1", ProductID: "p2", Unit: "szt", Amount: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := s.PantryItems(ctx, "hA")
	if len(items) != 1 || items[0].ProductID != "p2" || *items[0].Quantity != 4 {
		t.Fatalf("items: %+v", items)
	}
	ms, _ := s.MovementsForEntry(ctx, "hA", "e1")
	if len(ms) != 1 || ms[0].ProductID != "p2" || ms[0].Amount != 4 {
		t.Fatalf("movements: %+v", ms)
	}
	// Staple (nil quantity) rows are untouched and record nothing.
	st, _ := s.AddToPantry(ctx, "hA", "p1", "g", 1)
	st.Quantity = nil
	_ = s.SavePantryItem(ctx, st)
	_ = s.ApplyMovements(ctx, "hA", []domain.PantryMovement{{EntryID: "e1", ProductID: "p1", Unit: "g", Amount: -50}})
	got, _ := s.PantryItemByID(ctx, "hA", st.ID)
	ms, _ = s.MovementsForEntry(ctx, "hA", "e1")
	if got.Quantity != nil || len(ms) != 1 {
		t.Fatalf("staple: %+v %+v", got, ms)
	}
}

func TestApplyMovementsAtomic(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_ = s.InsertPlanEntry(ctx, entry("e1", "hA", "2026-11-02", domain.SlotDinner))
	err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{
		{EntryID: "e1", ProductID: "p1", Unit: "g", Amount: 100},
		{EntryID: "missing", ProductID: "p1", Unit: "g", Amount: 100},
	})
	if err == nil {
		t.Fatal("expected FK error")
	}
	if items, _ := s.PantryItems(ctx, "hA"); len(items) != 0 {
		t.Fatalf("not rolled back: %+v", items)
	}
}

func TestMovementsCascadeWithPlanEntry(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_ = s.InsertPlanEntry(ctx, entry("e1", "hA", "2026-11-02", domain.SlotDinner))
	_ = s.InsertPlanEntry(ctx, entry("e2", "hA", "2026-11-02", domain.SlotLunch))
	_, _ = s.AddToPantry(ctx, "hA", "p1", "g", 300)
	for _, e := range []string{"e1", "e2"} {
		if err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{{EntryID: e, ProductID: "p1", Unit: "g", Amount: -10}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeletePlanEntry(ctx, "hA", "e1"); err != nil {
		t.Fatal(err)
	}
	if ms, _ := s.MovementsForEntry(ctx, "hA", "e1"); len(ms) != 0 {
		t.Fatalf("cascade: %+v", ms)
	}
	if err := s.DeleteMovementsForEntry(ctx, "hA", "e2"); err != nil {
		t.Fatal(err)
	}
	if ms, _ := s.MovementsForEntry(ctx, "hA", "e2"); len(ms) != 0 {
		t.Fatalf("delete: %+v", ms)
	}
}

func TestPantryIsolationAndReview(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	it, _ := s.AddToPantry(ctx, "hA", "p1", "g", 5)
	if _, err := s.PantryItemByID(ctx, "hB", it.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	if items, _ := s.PantryItems(ctx, "hB"); len(items) != 0 {
		t.Fatalf("hB sees %+v", items)
	}
	if at, err := s.PantryReviewedAt(ctx, "hA"); err != nil || at != nil {
		t.Fatalf("initial: %v %v", err, at)
	}
	want := time.Unix(1_800_000_000, 0).UTC()
	if err := s.MarkPantryReviewed(ctx, "hA", want); err != nil {
		t.Fatal(err)
	}
	at, err := s.PantryReviewedAt(ctx, "hA")
	if err != nil || at == nil || !at.Equal(want) {
		t.Fatalf("after: %v %v", err, at)
	}
	if at, _ := s.PantryReviewedAt(ctx, "hB"); at != nil {
		t.Fatal("hB should be nil")
	}
}

func TestApplyMovementsRejectsForeignEntry(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_ = s.InsertPlanEntry(ctx, entry("eB", "hB", "2026-11-02", domain.SlotDinner))
	_, _ = s.AddToPantry(ctx, "hA", "p1", "g", 100)
	err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{{EntryID: "eB", ProductID: "p1", Unit: "g", Amount: -10}})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign entry: %v", err)
	}
	if ms, _ := s.MovementsForEntry(ctx, "hB", "eB"); len(ms) != 0 {
		t.Fatalf("recorded: %+v", ms)
	}
}

func TestApplyMovementsConcurrent(t *testing.T) {
	if os.Getenv("MEALPLAN_TEST_DRIVER") != "postgres" {
		t.Skip("Postgres-only concurrency test")
	}
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	it, _ := s.AddToPantry(ctx, "hA", "p1", "g", 300)
	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("e%d", i)
		if err := s.InsertPlanEntry(ctx, entry(id, "hA", "2026-11-02", domain.SlotDinner)); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{{EntryID: id, ProductID: "p1", Unit: "g", Amount: -10}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, _ := s.PantryItemByID(ctx, "hA", it.ID)
	sum := 0.0
	for i := 0; i < n; i++ {
		ms, _ := s.MovementsForEntry(ctx, "hA", fmt.Sprintf("e%d", i))
		for _, m := range ms {
			sum += m.Amount
		}
	}
	if *got.Quantity != 100 || sum != -200 {
		t.Fatalf("qty %v sum %v", *got.Quantity, sum)
	}
}

func TestConsumeRestoreForSession(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO prep_session (id, household_id, date, name, week, cooked, created_at) VALUES ('s1', 'hA', '2026-11-08', 'N', '2026-11-09', 0, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertPlanEntry(ctx, entry("e1", "hA", "2026-11-02", domain.SlotDinner)); err != nil {
		t.Fatal(err)
	}
	it, _ := s.AddToPantry(ctx, "hA", "p1", "g", 300)
	qty := func() float64 {
		g, _ := s.PantryItemByID(ctx, "hA", it.ID)
		return *g.Quantity
	}
	// An entry movement for the same product must not interfere with the session's.
	if ok, err := s.ConsumeForEntry(ctx, "hA", "e1", []domain.PantryMovement{{EntryID: "e1", ProductID: "p1", Unit: "g", Amount: -50}}); err != nil || !ok {
		t.Fatalf("entry consume: %v %v", ok, err)
	}
	ms := []domain.PantryMovement{{SessionID: "s1", ProductID: "p1", Unit: "g", Amount: -500}}
	if ok, err := s.ConsumeForSession(ctx, "hA", "s1", ms); err != nil || !ok {
		t.Fatalf("consume: %v %v", ok, err)
	}
	if q := qty(); q != 0 {
		t.Fatalf("clamped at zero: %v", q)
	}
	// Second consume is a no-op.
	if ok, err := s.ConsumeForSession(ctx, "hA", "s1", ms); err != nil || ok {
		t.Fatalf("second consume: %v %v", ok, err)
	}
	if q := qty(); q != 0 {
		t.Fatalf("after second consume: %v", q)
	}
	// Restore returns exactly what was applied (250), leaving the entry's -50 alone.
	if err := s.RestoreForSession(ctx, "hA", "s1"); err != nil {
		t.Fatal(err)
	}
	if q := qty(); q != 250 {
		t.Fatalf("after restore: %v", q)
	}
	if ms, _ := s.MovementsForEntry(ctx, "hA", "e1"); len(ms) != 1 {
		t.Fatalf("entry movement disturbed: %+v", ms)
	}
	if err := s.RestoreForSession(ctx, "hA", "s1"); err != nil {
		t.Fatal(err)
	}
	if q := qty(); q != 250 {
		t.Fatalf("restore twice: %v", q)
	}
	// Foreign household / wrong session id.
	if _, err := s.ConsumeForSession(ctx, "hB", "s1", ms); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign session: %v", err)
	}
	if _, err := s.ConsumeForSession(ctx, "hA", "s1", []domain.PantryMovement{{SessionID: "other", ProductID: "p1", Unit: "g", Amount: -1}}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mismatched session id: %v", err)
	}
}
