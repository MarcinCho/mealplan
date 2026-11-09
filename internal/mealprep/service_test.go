package mealprep_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/mealprep"
	"mealplan/internal/pantry"
	"mealplan/internal/product"
	"mealplan/internal/recipe"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func fp(v float64) *float64 { return &v }

func TestDefaultWeek(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-08": "2026-11-09", "2026-11-07": "2026-11-09", "2026-11-04": "2026-11-02",
		"2026-11-09": "2026-11-09", "2026-11-13": "2026-11-09",
	} {
		got, err := mealprep.DefaultWeek(in)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v want %q", in, got, err, want)
		}
	}
	if _, err := mealprep.DefaultWeek("2026-13-01"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

type fixture struct {
	st     *store.Store
	svc    *mealprep.Service
	recipe domain.Recipe
	prod   domain.Product
	hid    string
}

type failHook struct{}

func (failHook) SessionCooked(context.Context, string, domain.PrepSession, []domain.PrepDish) error {
	return errors.New("boom")
}
func (failHook) SessionUncooked(context.Context, string, string) error { return nil }

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st := store.New(storetest.Open(t))
	ctx := context.Background()
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: "h1", Name: "Dom"}, domain.User{ID: "u1", Login: "marc", PasswordHash: "x", DisplayName: "M"}, false); err != nil {
		t.Fatal(err)
	}
	prods := product.New(st, nil)
	prod, err := prods.Create(ctx, "h1", product.Input{Name: "Mąka testowa"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := recipe.New(st, nil).Create(ctx, "h1", recipe.Input{Title: "Chleb", BaseServings: 4,
		Ingredients: []recipe.IngredientInput{{Name: "Mąka testowa", Quantity: fp(500), Unit: "g", ProductID: prod.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	psvc := pantry.New(st, st, prods, nil)
	now := func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }
	return &fixture{st: st, svc: mealprep.New(st, st, psvc, now), recipe: r, prod: prod, hid: "h1"}
}

func (f *fixture) session(t *testing.T) mealprep.SessionView {
	t.Helper()
	s, err := f.svc.CreateSession(context.Background(), f.hid, mealprep.SessionInput{Date: "2026-11-08", Name: " Niedziela "})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fixture) dish(t *testing.T, sessionID string, portions int) mealprep.DishView {
	t.Helper()
	d, err := f.svc.AddDish(context.Background(), f.hid, sessionID, f.recipe.ID, portions)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// allocate plans servings of a dish or freezer item as a prep entry.
func (f *fixture) allocate(t *testing.T, id, dishID, freezerID string, servings int) domain.PlanEntry {
	t.Helper()
	e := domain.PlanEntry{ID: id, HouseholdID: f.hid, Date: "2026-11-10", Slot: domain.SlotDinner, RecipeID: f.recipe.ID,
		Servings: servings, Kind: domain.EntryPrep, PrepDishID: dishID, FreezerItemID: freezerID}
	if err := f.st.InsertPlanEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestCreateSessionDefaultsWeek(t *testing.T) {
	f := newFixture(t)
	s := f.session(t)
	if s.Week != "2026-11-09" || s.Name != "Niedziela" || s.Dishes == nil {
		t.Fatalf("%+v", s)
	}
	for _, in := range []mealprep.SessionInput{{Date: "2026-11-08", Name: " "}, {Date: "bad", Name: "x"}, {Date: "2026-11-08", Name: "x", Week: "2026-11-10"}} {
		if _, err := f.svc.CreateSession(context.Background(), f.hid, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%+v: want ErrInvalid, got %v", in, err)
		}
	}
}

func TestFreezeBeyondRemainingInvalid(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, f.session(t).ID, 6)
	f.allocate(t, "e1", d.ID, "", 2)
	if _, err := f.svc.Freeze(ctx, f.hid, d.ID, 5, ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
	if _, err := f.svc.Freeze(ctx, f.hid, d.ID, 0, ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
	fz, err := f.svc.Freeze(ctx, f.hid, d.ID, 4, "")
	if err != nil || fz.Label != "Chleb" || fz.FrozenOn != "2026-11-08" || fz.Remaining != 4 {
		t.Fatalf("%+v %v", fz, err)
	}
	s, _ := f.svc.Session(ctx, f.hid, d.SessionID)
	if dv := s.Dishes[0]; dv.Allocated != 2 || dv.Frozen != 4 || dv.Remaining != 0 || s.Remaining != 0 || len(dv.Allocations) != 1 || dv.RecipeTitle != "Chleb" {
		t.Fatalf("%+v", dv)
	}
}

func TestUpdateDishBelowUsedInvalid(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, f.session(t).ID, 6)
	f.allocate(t, "e1", d.ID, "", 2)
	if _, err := f.svc.Freeze(ctx, f.hid, d.ID, 2, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateDish(ctx, f.hid, d.ID, 3, 0); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("below used: %v", err)
	}
	if _, err := f.svc.UpdateDish(ctx, f.hid, d.ID, 6, 3); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("written off too big: %v", err)
	}
	if _, err := f.svc.UpdateDish(ctx, f.hid, d.ID, 201, 0); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("too many: %v", err)
	}
	dv, err := f.svc.UpdateDish(ctx, f.hid, d.ID, 6, 2)
	if err != nil || dv.Remaining != 0 || dv.WrittenOff != 2 {
		t.Fatalf("%+v %v", dv, err)
	}
}

func TestDeleteSessionWithAllocationInUse(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.session(t)
	d := f.dish(t, s.ID, 4)
	f.allocate(t, "e1", d.ID, "", 1)
	if err := f.svc.DeleteSession(ctx, f.hid, s.ID); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("session: %v", err)
	}
	if err := f.svc.DeleteDish(ctx, f.hid, d.ID); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("dish: %v", err)
	}
	if err := f.st.DeletePlanEntry(ctx, f.hid, "e1"); err != nil {
		t.Fatal(err)
	}
	fz, err := f.svc.Freeze(ctx, f.hid, d.ID, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteSession(ctx, f.hid, s.ID); err != nil {
		t.Fatal(err)
	}
	items, _ := f.svc.Freezer(ctx, f.hid)
	if len(items) != 1 || items[0].ID != fz.ID {
		t.Fatalf("frozen item should survive: %+v", items)
	}
	f.allocate(t, "e2", "", fz.ID, 1)
	if err := f.svc.DeleteFreezer(ctx, f.hid, fz.ID); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("freezer: %v", err)
	}
}

func (f *fixture) flour(t *testing.T) float64 {
	t.Helper()
	items, _ := f.st.PantryItems(context.Background(), f.hid)
	for _, it := range items {
		if it.ProductID == f.prod.ID && it.Quantity != nil {
			return *it.Quantity
		}
	}
	return -1
}

func TestCookingSessionDeductsAndRestores(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.st.AddToPantry(ctx, f.hid, f.prod.ID, "g", 1000); err != nil {
		t.Fatal(err)
	}
	s := f.session(t)
	f.dish(t, s.ID, 8)
	in := mealprep.SessionInput{Date: s.Date, Name: s.Name, Week: s.Week, Cooked: true}
	step := func(name string, cooked bool, want float64) {
		t.Helper()
		in.Cooked = cooked
		if _, err := f.svc.UpdateSession(ctx, f.hid, s.ID, in); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := f.flour(t); got != want {
			t.Fatalf("%s: %v g, want %v", name, got, want)
		}
	}
	step("cook", true, 0)
	step("cook again", true, 0)
	step("uncook", false, 1000)
	step("uncook again", false, 1000)
}

func TestCookHookFailureKeepsState(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := mealprep.New(f.st, f.st, failHook{}, nil)
	s, err := svc.CreateSession(ctx, f.hid, mealprep.SessionInput{Date: "2026-11-08", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateSession(ctx, f.hid, s.ID, mealprep.SessionInput{Date: s.Date, Name: s.Name, Cooked: true})
	if !errors.Is(err, mealprep.ErrPantry) {
		t.Fatalf("want ErrPantry, got %v", err)
	}
	if got, _ := svc.Session(ctx, f.hid, s.ID); !got.Cooked {
		t.Fatal("cooked state should be kept")
	}
}

func TestAvailableCountsOwnEntry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, f.session(t).ID, 6)
	f.allocate(t, "e1", d.ID, "", 4)
	rid, label, rem, err := f.svc.Available(ctx, f.hid, d.ID, "", "")
	if err != nil || rem != 2 || rid != f.recipe.ID || label != "Chleb" {
		t.Fatalf("%q %q %d %v", rid, label, rem, err)
	}
	if _, _, rem, _ = f.svc.Available(ctx, f.hid, d.ID, "", "e1"); rem != 6 {
		t.Fatalf("except own: %d", rem)
	}
	if _, _, _, err = f.svc.Available(ctx, f.hid, "", "", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
	fz, err := f.svc.AddFreezer(ctx, f.hid, mealprep.FreezerInput{Label: "Rosół", Portions: 3})
	if err != nil {
		t.Fatal(err)
	}
	f.allocate(t, "e2", "", fz.ID, 2)
	if _, label, rem, _ = f.svc.Available(ctx, f.hid, "", fz.ID, "e2"); rem != 3 || label != "Rosół" {
		t.Fatalf("%q %d", label, rem)
	}
}

func TestFreezerListHidesEmpty(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, err := f.svc.AddFreezer(ctx, f.hid, mealprep.FreezerInput{RecipeID: f.recipe.ID, Portions: 2})
	if err != nil || a.Label != "Chleb" || a.FrozenOn != "2026-11-08" {
		t.Fatalf("%+v %v", a, err)
	}
	b, _ := f.svc.AddFreezer(ctx, f.hid, mealprep.FreezerInput{Label: "Zupa", Portions: 1, FrozenOn: "2026-10-01"})
	f.allocate(t, "e1", "", b.ID, 1)
	list, err := f.svc.Freezer(ctx, f.hid)
	if err != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("%+v %v", list, err)
	}
	if _, err := f.svc.UpdateFreezer(ctx, f.hid, b.ID, mealprep.FreezerInput{Label: "Zupa", Portions: 0}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("%v", err)
	}
	if _, err := f.svc.AddFreezer(ctx, f.hid, mealprep.FreezerInput{Portions: 1}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no recipe/label: %v", err)
	}
	if _, err := f.svc.UpdateFreezer(ctx, f.hid, b.ID, mealprep.FreezerInput{Label: "Zupa", Portions: 2, FrozenOn: "2026-10-01"}); err != nil {
		t.Fatal(err)
	}
}

func TestPortionsPerMealRange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, n := range []int{0, 21} {
		if err := f.svc.SetPortionsPerMeal(ctx, f.hid, n); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("%d: %v", n, err)
		}
	}
	if err := f.svc.SetPortionsPerMeal(ctx, f.hid, 3); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.svc.PortionsPerMeal(ctx, f.hid); n != 3 {
		t.Fatal(n)
	}
}

func TestUpdateFreezerRespectsDishAndKeepsRecipe(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.session(t)
	d := f.dish(t, s.ID, 6)
	fz, err := f.svc.Freeze(ctx, f.hid, d.ID, 4, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateFreezer(ctx, f.hid, fz.ID, mealprep.FreezerInput{Label: fz.Label, Portions: 7, FrozenOn: fz.FrozenOn}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
	got, err := f.svc.UpdateFreezer(ctx, f.hid, fz.ID, mealprep.FreezerInput{Label: fz.Label, Portions: 6, FrozenOn: fz.FrozenOn})
	if err != nil || got.RecipeID != f.recipe.ID || got.DishID != d.ID {
		t.Fatalf("%+v %v", got, err)
	}
	sv, _ := f.svc.Session(ctx, f.hid, s.ID)
	if sv.Dishes[0].Remaining != 0 {
		t.Fatalf("dish remaining %d", sv.Dishes[0].Remaining)
	}
	got, err = f.svc.UpdateFreezer(ctx, f.hid, fz.ID, mealprep.FreezerInput{Label: "Inna", Portions: 5})
	if err != nil || got.RecipeID != f.recipe.ID || got.Label != "Inna" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestFreezeFrozenOn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, f.session(t).ID, 6)
	if _, err := f.svc.Freeze(ctx, f.hid, d.ID, 1, "08.11.2026"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad date: %v", err)
	}
	fz, err := f.svc.Freeze(ctx, f.hid, d.ID, 1, "2026-11-09")
	if err != nil || fz.FrozenOn != "2026-11-09" {
		t.Fatalf("%+v %v", fz, err)
	}
	fz, err = f.svc.Freeze(ctx, f.hid, d.ID, 1, "")
	if err != nil || fz.FrozenOn != "2026-11-08" {
		t.Fatalf("default: %+v %v", fz, err)
	}
}

func TestCookedSessionFollowsDishChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.st.AddToPantry(ctx, f.hid, f.prod.ID, "g", 3000); err != nil {
		t.Fatal(err)
	}
	s := f.session(t)
	first := f.dish(t, s.ID, 4) // 500 g of flour per 4 portions
	if _, err := f.svc.UpdateSession(ctx, f.hid, s.ID, mealprep.SessionInput{Date: s.Date, Name: s.Name, Week: s.Week, Cooked: true}); err != nil {
		t.Fatal(err)
	}
	check := func(name string, want float64) {
		t.Helper()
		if got := f.flour(t); got != want {
			t.Fatalf("%s: %v g, want %v", name, got, want)
		}
	}
	check("cooked", 2500)
	second := f.dish(t, s.ID, 4)
	check("dish added", 2000)
	if _, err := f.svc.UpdateDish(ctx, f.hid, first.ID, 8, 0); err != nil {
		t.Fatal(err)
	}
	check("dish resized", 1500)
	if _, err := f.svc.UpdateDish(ctx, f.hid, first.ID, 8, 2); err != nil {
		t.Fatal(err)
	}
	check("written off does not touch stock", 1500)
	if err := f.svc.DeleteDish(ctx, f.hid, second.ID); err != nil {
		t.Fatal(err)
	}
	check("dish removed", 2000)
	if err := f.svc.DeleteSession(ctx, f.hid, s.ID); err != nil {
		t.Fatal(err)
	}
	check("session deleted", 3000)
}

func TestUncookedSessionDishChangesLeaveStock(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.st.AddToPantry(ctx, f.hid, f.prod.ID, "g", 3000); err != nil {
		t.Fatal(err)
	}
	s := f.session(t)
	d := f.dish(t, s.ID, 4)
	if _, err := f.svc.UpdateDish(ctx, f.hid, d.ID, 8, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteSession(ctx, f.hid, s.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.flour(t); got != 3000 {
		t.Fatalf("stock changed: %v", got)
	}
}
