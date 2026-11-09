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

func TestPlanEntryKindsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	db := s.DB()
	for _, q := range []string{
		`INSERT INTO prep_session (id, household_id, date, name, week, cooked, created_at) VALUES ('s1', 'hA', '2026-11-08', 'Niedziela', '2026-11-09', 0, 1)`,
		`INSERT INTO prep_dish (id, household_id, session_id, recipe_id, portions, written_off, position) VALUES ('d1', 'hA', 's1', 'r1', 6, 0, 0)`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	prep := domain.PlanEntry{ID: "ep", HouseholdID: "hA", Date: "2026-11-10", Slot: domain.SlotDinner, RecipeID: "r1", Servings: 2, Kind: domain.EntryPrep, PrepDishID: "d1"}
	if err := s.InsertPlanEntry(ctx, prep); err != nil {
		t.Fatal(err)
	}
	got, err := s.PlanEntryByID(ctx, "hA", "ep")
	if err != nil || got != prep {
		t.Fatalf("prep: %v %+v", err, got)
	}
	out := domain.PlanEntry{ID: "eo", HouseholdID: "hA", Date: "2026-11-10", Slot: domain.SlotLunch, Servings: 1, Kind: domain.EntryOut,
		OutValues: domain.Macros{Kcal: f(800), Protein: f(30), Carbs: f(90), Fat: f(35)}}
	if err := s.InsertPlanEntry(ctx, out); err != nil {
		t.Fatal(err)
	}
	nilOut := domain.PlanEntry{ID: "en", HouseholdID: "hA", Date: "2026-11-10", Slot: domain.SlotSnack, Servings: 1, Kind: domain.EntryOut}
	if err := s.InsertPlanEntry(ctx, nilOut); err != nil {
		t.Fatal(err)
	}
	list, err := s.PlanEntries(ctx, "hA", "2026-11-10", "2026-11-10")
	if err != nil || len(list) != 3 {
		t.Fatalf("list: %v %+v", err, list)
	}
	byID := map[string]domain.PlanEntry{}
	for _, e := range list {
		byID[e.ID] = e
	}
	o := byID["eo"].OutValues
	if o.Kcal == nil || *o.Kcal != 800 || *o.Protein != 30 || *o.Carbs != 90 || *o.Fat != 35 {
		t.Fatalf("out values: %+v", o)
	}
	if n := byID["en"].OutValues; n.Kcal != nil || n.Protein != nil || n.Carbs != nil || n.Fat != nil {
		t.Fatalf("nil out values: %+v", n)
	}
	// Update flips values and keeps the kind.
	out.OutValues = domain.Macros{Kcal: f(500)}
	if err := s.UpdatePlanEntry(ctx, out); err != nil {
		t.Fatal(err)
	}
	if g, _ := s.PlanEntryByID(ctx, "hA", "eo"); g.OutValues.Kcal == nil || *g.OutValues.Kcal != 500 || g.OutValues.Protein != nil {
		t.Fatalf("after update: %+v", g.OutValues)
	}
}
