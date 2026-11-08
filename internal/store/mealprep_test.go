package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func sess(id, hid, date, week string) domain.PrepSession {
	return domain.PrepSession{ID: id, HouseholdID: hid, Date: date, Name: "Niedziela", Week: week, CreatedAt: time.Unix(1000, 0).UTC()}
}

func TestPrepSessionCRUD(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	a := sess("s1", "hA", "2026-11-08", "2026-11-09")
	b := sess("s2", "hA", "2026-11-15", "2026-11-16")
	c := sess("s3", "hA", "2026-11-15", "2026-11-16")
	c.CreatedAt = time.Unix(2000, 0).UTC()
	for _, x := range []domain.PrepSession{a, b, c} {
		if err := s.InsertPrepSession(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.PrepSessionByID(ctx, "hA", "s1")
	if err != nil || got != a {
		t.Fatalf("get: %v %+v", err, got)
	}
	all, err := s.PrepSessions(ctx, "hA")
	if err != nil || len(all) != 3 || all[0].ID != "s3" || all[1].ID != "s2" || all[2].ID != "s1" {
		t.Fatalf("order: %v %+v", err, all)
	}
	wk, err := s.PrepSessionsForWeek(ctx, "hA", "2026-11-16")
	if err != nil || len(wk) != 2 {
		t.Fatalf("week: %v %+v", err, wk)
	}
	a.Name, a.Cooked, a.Date, a.Week = "Zmiana", true, "2026-11-09", "2026-11-09"
	if err := s.UpdatePrepSession(ctx, a); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.PrepSessionByID(ctx, "hA", "s1"); got != a {
		t.Fatalf("after update: %+v", got)
	}
	// foreign household
	if _, err := s.PrepSessionByID(ctx, "hB", "s1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	a.HouseholdID = "hB"
	if err := s.UpdatePrepSession(ctx, a); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if err := s.DeletePrepSession(ctx, "hB", "s1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if err := s.DeletePrepSession(ctx, "hA", "s1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePrepSession(ctx, "hA", "s1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("double delete: %v", err)
	}
}

func TestPrepDishOrderAndScope(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	_ = s.CreateRecipe(ctx, sampleRecipe("r2", "hB"))
	_ = s.InsertPrepSession(ctx, sess("s1", "hA", "2026-11-08", "2026-11-09"))
	_ = s.InsertPrepSession(ctx, sess("s2", "hA", "2026-11-09", "2026-11-09"))
	for _, d := range []domain.PrepDish{
		{ID: "d1", HouseholdID: "hA", SessionID: "s1", RecipeID: "r1", Portions: 4},
		{ID: "d2", HouseholdID: "hA", SessionID: "s1", RecipeID: "r1", Portions: 6},
		{ID: "d3", HouseholdID: "hA", SessionID: "s2", RecipeID: "r1", Portions: 2},
	} {
		if err := s.InsertPrepDish(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if d, _ := s.PrepDishByID(ctx, "hA", "d2"); d.Position != 1 || d.Portions != 6 {
		t.Fatalf("d2: %+v", d)
	}
	if d, _ := s.PrepDishByID(ctx, "hA", "d3"); d.Position != 0 {
		t.Fatalf("d3 position: %+v", d)
	}
	list, err := s.PrepDishes(ctx, "hA", []string{"s1", "s2"})
	if err != nil || len(list) != 3 || list[0].ID != "d1" || list[1].ID != "d2" || list[2].ID != "d3" {
		t.Fatalf("list: %v %+v", err, list)
	}
	if list, err := s.PrepDishes(ctx, "hA", nil); err != nil || len(list) != 0 {
		t.Fatalf("empty: %v %+v", err, list)
	}
	if list, _ := s.PrepDishes(ctx, "hB", []string{"s1"}); len(list) != 0 {
		t.Fatalf("foreign list: %+v", list)
	}
	// foreign household cannot read/update/delete
	if _, err := s.PrepDishByID(ctx, "hB", "d1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	if err := s.UpdatePrepDish(ctx, domain.PrepDish{ID: "d1", HouseholdID: "hB", Portions: 9}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if err := s.DeletePrepDish(ctx, "hB", "d1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	// cannot attach to a foreign session or recipe
	if err := s.InsertPrepDish(ctx, domain.PrepDish{ID: "x", HouseholdID: "hB", SessionID: "s1", RecipeID: "r2", Portions: 1}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign session: %v", err)
	}
	if err := s.InsertPrepDish(ctx, domain.PrepDish{ID: "x", HouseholdID: "hA", SessionID: "s1", RecipeID: "r2", Portions: 1}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign recipe: %v", err)
	}
	// update
	d, _ := s.PrepDishByID(ctx, "hA", "d1")
	d.Portions, d.WrittenOff = 5, 1
	if err := s.UpdatePrepDish(ctx, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PrepDishByID(ctx, "hA", "d1"); got.Portions != 5 || got.WrittenOff != 1 || got.Position != 0 {
		t.Fatalf("after update: %+v", got)
	}
	if err := s.DeletePrepDish(ctx, "hA", "d1"); err != nil {
		t.Fatal(err)
	}
	// deleting a session cascades to its dishes
	if err := s.DeletePrepSession(ctx, "hA", "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepDishByID(ctx, "hA", "d2"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cascade: %v", err)
	}
}

func TestFreezerItemCRUD(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	_ = s.InsertPrepSession(ctx, sess("s1", "hA", "2026-11-08", "2026-11-09"))
	_ = s.InsertPrepDish(ctx, domain.PrepDish{ID: "d1", HouseholdID: "hA", SessionID: "s1", RecipeID: "r1", Portions: 4})
	a := domain.FreezerItem{ID: "f1", HouseholdID: "hA", DishID: "d1", RecipeID: "r1", Label: "Gulasz", Portions: 3, FrozenOn: "2026-11-10"}
	b := domain.FreezerItem{ID: "f2", HouseholdID: "hA", Label: "Zupa", Portions: 2, FrozenOn: "2026-11-01"}
	for _, x := range []domain.FreezerItem{a, b} {
		if err := s.InsertFreezerItem(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.FreezerItemByID(ctx, "hA", "f1"); err != nil || got != a {
		t.Fatalf("get: %v %+v", err, got)
	}
	if got, err := s.FreezerItemByID(ctx, "hA", "f2"); err != nil || got != b {
		t.Fatalf("get nullable: %v %+v", err, got)
	}
	list, err := s.FreezerItems(ctx, "hA")
	if err != nil || len(list) != 2 || list[0].ID != "f2" {
		t.Fatalf("list: %v %+v", err, list)
	}
	a.Portions = 1
	if err := s.UpdateFreezerItem(ctx, a); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.FreezerItemByID(ctx, "hA", "f1"); got.Portions != 1 {
		t.Fatalf("update: %+v", got)
	}
	if _, err := s.FreezerItemByID(ctx, "hB", "f1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	if err := s.DeleteFreezerItem(ctx, "hB", "f1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	a.HouseholdID = "hB"
	if err := s.UpdateFreezerItem(ctx, a); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if err := s.InsertFreezerItem(ctx, domain.FreezerItem{ID: "f9", HouseholdID: "hB", DishID: "d1", Label: "x", Portions: 1, FrozenOn: "2026-11-10"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign dish: %v", err)
	}
	// deleting the dish keeps the item and its label
	if err := s.DeletePrepDish(ctx, "hA", "d1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.FreezerItemByID(ctx, "hA", "f1")
	if err != nil || got.DishID != "" || got.Label != "Gulasz" {
		t.Fatalf("after dish delete: %v %+v", err, got)
	}
	if err := s.DeleteFreezerItem(ctx, "hA", "f1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFreezerItem(ctx, "hA", "f1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("double delete: %v", err)
	}
}

func TestAllocationsListsOnlyPrepEntries(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	_ = s.InsertPrepSession(ctx, sess("s1", "hA", "2026-11-08", "2026-11-09"))
	_ = s.InsertPrepDish(ctx, domain.PrepDish{ID: "d1", HouseholdID: "hA", SessionID: "s1", RecipeID: "r1", Portions: 4})
	_ = s.InsertFreezerItem(ctx, domain.FreezerItem{ID: "f1", HouseholdID: "hA", Label: "Zupa", Portions: 2, FrozenOn: "2026-11-01"})
	for _, e := range []domain.PlanEntry{
		{ID: "e1", HouseholdID: "hA", Date: "2026-11-10", Slot: domain.SlotDinner, FreeText: "Pizza", Servings: 2},
		{ID: "e2", HouseholdID: "hA", Date: "2026-11-11", Slot: domain.SlotDinner, RecipeID: "r1", Kind: domain.EntryPrep, PrepDishID: "d1", Servings: 2},
		{ID: "e3", HouseholdID: "hA", Date: "2027-03-01", Slot: domain.SlotLunch, FreeText: "Zupa", Kind: domain.EntryPrep, FreezerItemID: "f1", Servings: 1},
		{ID: "e4", HouseholdID: "hA", Date: "2026-11-12", Slot: domain.SlotLunch, Kind: domain.EntryOut, Servings: 1, OutValues: domain.Macros{Kcal: f(500)}},
	} {
		if err := s.InsertPlanEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Allocations(ctx, "hA")
	if err != nil || len(got) != 2 {
		t.Fatalf("allocations: %v %+v", err, got)
	}
	ids := map[string]domain.PlanEntry{got[0].ID: got[0], got[1].ID: got[1]}
	if ids["e2"].PrepDishID != "d1" || ids["e2"].Kind != domain.EntryPrep || ids["e3"].FreezerItemID != "f1" {
		t.Fatalf("columns: %+v", got)
	}
	if other, _ := s.Allocations(ctx, "hB"); len(other) != 0 {
		t.Fatalf("foreign: %+v", other)
	}
}

func TestDeleteRecipeUsedByDishIsInUse(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	_ = s.InsertPrepSession(ctx, sess("s1", "hA", "2026-11-08", "2026-11-09"))
	_ = s.InsertPrepDish(ctx, domain.PrepDish{ID: "d1", HouseholdID: "hA", SessionID: "s1", RecipeID: "r1", Portions: 4})
	if err := s.DeleteRecipe(ctx, "hA", "r1"); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("want ErrInUse, got %v", err)
	}
	if _, err := s.RecipeByID(ctx, "hA", "r1"); err != nil {
		t.Fatalf("recipe must survive: %v", err)
	}
}
