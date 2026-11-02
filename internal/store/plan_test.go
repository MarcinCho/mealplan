package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func entry(id, hid, date string, slot domain.Slot) domain.PlanEntry {
	return domain.PlanEntry{ID: id, HouseholdID: hid, Date: date, Slot: slot, FreeText: "Resztki", Servings: 2}
}

func TestPlanEntryLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))

	e := domain.PlanEntry{ID: "e1", HouseholdID: "hA", Date: "2026-11-02", Slot: domain.SlotDinner, RecipeID: "r1", Servings: 3, CookUserID: "uA"}
	if err := s.InsertPlanEntry(ctx, e); err != nil {
		t.Fatal(err)
	}
	got, err := s.PlanEntryByID(ctx, "hA", "e1")
	if err != nil || got.RecipeID != "r1" || got.CookUserID != "uA" || got.Servings != 3 || got.FreeText != "" {
		t.Fatalf("get: %v %+v", err, got)
	}
	e.RecipeID, e.FreeText, e.CookUserID, e.Servings = "", "Pizza", "", 4
	if err := s.UpdatePlanEntry(ctx, e); err != nil {
		t.Fatal(err)
	}
	got, _ = s.PlanEntryByID(ctx, "hA", "e1")
	if got.RecipeID != "" || got.FreeText != "Pizza" || got.CookUserID != "" || got.Servings != 4 {
		t.Fatalf("after update: %+v", got)
	}
	if err := s.DeletePlanEntry(ctx, "hA", "e1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePlanEntry(ctx, "hA", "e1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("double delete: %v", err)
	}
}

func TestPlanEntryDoneRoundTrips(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	e := entry("e1", "hA", "2026-11-02", domain.SlotDinner)
	if err := s.InsertPlanEntry(ctx, e); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PlanEntryByID(ctx, "hA", "e1"); got.Done {
		t.Fatalf("new entries start undone: %+v", got)
	}
	e.Done = true
	if err := s.UpdatePlanEntry(ctx, e); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PlanEntryByID(ctx, "hA", "e1"); !got.Done {
		t.Fatalf("done not stored: %+v", got)
	}
}

func TestPlanEntriesRangeAndOrder(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	for _, e := range []domain.PlanEntry{
		entry("e3", "hA", "2026-11-03", domain.SlotLunch),
		entry("e1", "hA", "2026-11-02", domain.SlotLunch),
		entry("e2", "hA", "2026-11-02", domain.SlotBreakfast),
		entry("e9", "hA", "2026-11-09", domain.SlotLunch),
		entry("eB", "hB", "2026-11-02", domain.SlotLunch),
	} {
		if err := s.InsertPlanEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.PlanEntries(ctx, "hA", "2026-11-02", "2026-11-08")
	if err != nil || len(got) != 3 {
		t.Fatalf("range: %v %+v", err, got)
	}
	if got[0].ID != "e1" && got[0].ID != "e2" || got[2].ID != "e3" {
		t.Fatalf("order: %+v", got)
	}
	for _, e := range got {
		if e.HouseholdID != "hA" {
			t.Fatalf("leak: %+v", e)
		}
	}
}

func TestPlanEntryRejectsForeignReferences(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("rB", "hB"))

	foreignRecipe := domain.PlanEntry{ID: "e1", HouseholdID: "hA", Date: "2026-11-02", Slot: domain.SlotLunch, RecipeID: "rB", Servings: 2}
	if err := s.InsertPlanEntry(ctx, foreignRecipe); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign recipe: %v", err)
	}
	foreignCook := entry("e2", "hA", "2026-11-02", domain.SlotLunch)
	foreignCook.CookUserID = "uB"
	if err := s.InsertPlanEntry(ctx, foreignCook); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign cook: %v", err)
	}
	if err := s.InsertPlanEntry(ctx, entry("e3", "hA", "2026-11-02", domain.SlotLunch)); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlanEntry(ctx, entry("e3", "hB", "2026-11-02", domain.SlotLunch)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update foreign entry: %v", err)
	}
	if err := s.DeletePlanEntry(ctx, "hB", "e3"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete foreign entry: %v", err)
	}
	if _, err := s.PlanEntryByID(ctx, "hB", "e3"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get foreign entry: %v", err)
	}
}

func TestCopyPlanEntriesAtomicAndRefusesNonEmptyTarget(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	batch := []domain.PlanEntry{entry("c1", "hA", "2026-11-09", domain.SlotLunch), entry("c2", "hA", "2026-11-10", domain.SlotDinner)}
	if err := s.CopyPlanEntries(ctx, "hA", batch); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PlanEntries(ctx, "hA", "2026-11-09", "2026-11-15"); len(got) != 2 {
		t.Fatalf("copied: %+v", got)
	}
	again := []domain.PlanEntry{entry("c3", "hA", "2026-11-11", domain.SlotLunch)}
	if err := s.CopyPlanEntries(ctx, "hA", again); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("non-empty target must conflict, got %v", err)
	}
	broken := []domain.PlanEntry{entry("c4", "hA", "2026-11-16", domain.SlotLunch), {ID: "c5", HouseholdID: "hA", Date: "2026-11-17", Slot: "bogus", FreeText: "x", Servings: 1}}
	if err := s.CopyPlanEntries(ctx, "hA", broken); err == nil {
		t.Fatal("invalid batch must fail")
	}
	if got, _ := s.PlanEntries(ctx, "hA", "2026-11-16", "2026-11-22"); len(got) != 0 {
		t.Fatalf("partial copy leaked: %+v", got)
	}
}
