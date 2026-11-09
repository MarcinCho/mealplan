package plan_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/mealprep"
	"mealplan/internal/plan"
	"mealplan/internal/recipe"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

type fixture struct {
	st     *store.Store
	mp     *mealprep.Service
	svc    *plan.Service
	recipe domain.Recipe
	hid    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st := store.New(storetest.Open(t))
	ctx := context.Background()
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: "h1", Name: "Dom"}, domain.User{ID: "u1", Login: "marc", PasswordHash: "x", DisplayName: "M"}, false); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: "h2", Name: "Inny"}, domain.User{ID: "u2", Login: "eve", PasswordHash: "x", DisplayName: "E"}, false); err != nil {
		t.Fatal(err)
	}
	rs := recipe.New(st, func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) })
	r, err := rs.Create(ctx, "h1", recipe.Input{Title: "Zupa", BaseServings: 4, Ingredients: []recipe.IngredientInput{{Name: "Woda"}}})
	if err != nil {
		t.Fatal(err)
	}
	mp := mealprep.New(st, st, nil, nil)
	svc := plan.New(st, st)
	svc.SetPortions(mp)
	return &fixture{st: st, mp: mp, svc: svc, recipe: r, hid: "h1"}
}

const wk = "2026-11-02"

func TestAddDefaultsServingsFromRecipe(t *testing.T) {
	f := newFixture(t)
	e, err := f.svc.Add(context.Background(), f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, RecipeID: f.recipe.ID})
	if err != nil || e.Servings != 4 || e.ID == "" {
		t.Fatalf("add: %v %+v", err, e)
	}
	free, err := f.svc.Add(context.Background(), f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotSnack, FreeText: "  Owoce "})
	if err != nil || free.Servings != 1 || free.FreeText != "Owoce" {
		t.Fatalf("free text: %v %+v", err, free)
	}
}

func TestAddValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cases := map[string]plan.EntryInput{
		"date before week":        {Date: "2026-11-01", Slot: domain.SlotLunch, FreeText: "x"},
		"date after week":         {Date: "2026-11-09", Slot: domain.SlotLunch, FreeText: "x"},
		"malformed date":          {Date: "2026-11-4", Slot: domain.SlotLunch, FreeText: "x"},
		"bad slot":                {Date: "2026-11-04", Slot: "brunch", FreeText: "x"},
		"neither recipe nor text": {Date: "2026-11-04", Slot: domain.SlotLunch},
		"both recipe and text":    {Date: "2026-11-04", Slot: domain.SlotLunch, RecipeID: f.recipe.ID, FreeText: "x"},
		"negative servings":       {Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "x", Servings: -1},
		"huge servings":           {Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "x", Servings: 101},
		"nul in text":             {Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "a\x00b"},
		"control in text":         {Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "a\x1bb"},
		"long text":               {Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: string(make([]byte, 301))},
	}
	for name, in := range cases {
		if _, err := f.svc.Add(ctx, f.hid, wk, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: expected ErrInvalid, got %v", name, err)
		}
	}
	if _, err := f.svc.Add(ctx, f.hid, "2026-11-03", plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "x"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("non-monday week: %v", err)
	}
}

func TestForeignReferencesAreNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Add(ctx, "h2", wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotLunch, RecipeID: f.recipe.ID}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("recipe of another household: %v", err)
	}
	if _, err := f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "x", CookUserID: "u2"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cook of another household: %v", err)
	}
}

func TestEditMovesEntryAndKeepsItInWeek(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	e, _ := f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotLunch, FreeText: "x"})
	moved, err := f.svc.Edit(ctx, f.hid, wk, e.ID, plan.EntryInput{Date: "2026-11-05", Slot: domain.SlotDinner, FreeText: "y", Servings: 3, CookUserID: "u1"})
	if err != nil || moved.Date != "2026-11-05" || moved.CookUserID != "u1" || moved.Servings != 3 {
		t.Fatalf("edit: %v %+v", err, moved)
	}
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, plan.EntryInput{Date: "2026-11-12", Slot: domain.SlotDinner, FreeText: "y"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("move out of week: %v", err)
	}
	if _, err := f.svc.Edit(ctx, "h2", wk, e.ID, plan.EntryInput{Date: "2026-11-05", Slot: domain.SlotDinner, FreeText: "y"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("edit foreign: %v", err)
	}
}

func TestWeekReturnsOnlyThatWeek(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, _ = f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-08", Slot: domain.SlotLunch, FreeText: "niedziela"})
	_, _ = f.svc.Add(ctx, f.hid, "2026-11-09", plan.EntryInput{Date: "2026-11-09", Slot: domain.SlotLunch, FreeText: "kolejny"})
	got, err := f.svc.Week(ctx, f.hid, wk)
	if err != nil || len(got) != 1 || got[0].FreeText != "niedziela" {
		t.Fatalf("week: %v %+v", err, got)
	}
	if _, err := f.svc.Week(ctx, f.hid, "2026-11-04"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad week: %v", err)
	}
}

func TestCopyShiftsDatesAndRefusesNonEmptyTarget(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, _ = f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-03", Slot: domain.SlotLunch, RecipeID: f.recipe.ID, Servings: 2, CookUserID: "u1"})
	_, _ = f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-08", Slot: domain.SlotDinner, FreeText: "Pizza"})

	week, _ := f.svc.Week(ctx, f.hid, wk)
	if _, err := f.svc.Edit(ctx, f.hid, wk, week[0].ID, plan.EntryInput{Date: week[0].Date, Slot: week[0].Slot, RecipeID: week[0].RecipeID, Servings: 2, CookUserID: "u1", Done: true}); err != nil {
		t.Fatal(err)
	}

	n, err := f.svc.Copy(ctx, f.hid, wk, "2026-11-09")
	if err != nil || n != 2 {
		t.Fatalf("copy: %d %v", n, err)
	}
	got, _ := f.svc.Week(ctx, f.hid, "2026-11-09")
	if len(got) != 2 || got[0].Date != "2026-11-10" || got[1].Date != "2026-11-15" || got[0].CookUserID != "u1" || got[0].Servings != 2 || got[0].Done {
		t.Fatalf("copied week: %+v", got)
	}
	if _, err := f.svc.Copy(ctx, f.hid, wk, "2026-11-09"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("second copy must conflict: %v", err)
	}
	if _, err := f.svc.Copy(ctx, f.hid, wk, wk); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("copy onto itself: %v", err)
	}
	if _, err := f.svc.Copy(ctx, f.hid, wk, "2026-11-10"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("non-monday target: %v", err)
	}
}

func TestEditAndRemoveRespectTheWeek(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	const other = "2026-11-09"
	e, err := f.svc.Add(ctx, f.hid, other, plan.EntryInput{Date: "2026-11-10", Slot: domain.SlotLunch, FreeText: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, plan.EntryInput{Date: "2026-11-05", Slot: domain.SlotDinner, FreeText: "y"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("edit through wrong week: %v", err)
	}
	if err := f.svc.Remove(ctx, f.hid, wk, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("remove through wrong week: %v", err)
	}
	if err := f.svc.Remove(ctx, "h2", other, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("remove foreign: %v", err)
	}
	if _, err := f.svc.Edit(ctx, f.hid, other, e.ID, plan.EntryInput{Date: "2026-11-11", Slot: domain.SlotDinner, FreeText: "y"}); err != nil {
		t.Fatalf("edit through own week: %v", err)
	}
	if err := f.svc.Remove(ctx, f.hid, other, e.ID); err != nil {
		t.Fatalf("remove through own week: %v", err)
	}
}

type fakeHook struct {
	done, undone int
	failNext     bool
}

func (h *fakeHook) EntryDone(context.Context, string, domain.PlanEntry) error {
	h.done++
	if h.failNext {
		h.failNext = false
		return errors.New("boom")
	}
	return nil
}
func (h *fakeHook) EntryUndone(context.Context, string, string) error { h.undone++; return nil }

func TestHookIsStateBasedOnAddAndEdit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	in := plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, RecipeID: f.recipe.ID}
	// no hook: everything still works
	e, err := f.svc.Add(ctx, f.hid, wk, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Done = true
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); err != nil {
		t.Fatal(err)
	}
	h := &fakeHook{}
	f.svc.SetDoneHook(h)
	if _, err := f.svc.Add(ctx, f.hid, wk, in); err != nil || h.done != 1 {
		t.Fatalf("add done=true: %v done=%d", err, h.done)
	}
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); err != nil || h.done != 2 {
		t.Fatalf("edit done=true: %v done=%d", err, h.done)
	}
	in.Done = false
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); err != nil || h.undone != 1 {
		t.Fatalf("edit done=false: %v undone=%d", err, h.undone)
	}
}

func TestHookFailureIsReportedAndRetryCallsAgain(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	in := plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, RecipeID: f.recipe.ID}
	e, _ := f.svc.Add(ctx, f.hid, wk, in)
	h := &fakeHook{failNext: true}
	f.svc.SetDoneHook(h)
	in.Done = true
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); !errors.Is(err, plan.ErrPantry) {
		t.Fatalf("want ErrPantry, got %v", err)
	}
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); err != nil || h.done != 2 {
		t.Fatalf("retry: %v done=%d", err, h.done)
	}
}

func (f *fixture) dish(t *testing.T, portions int) string {
	t.Helper()
	ctx := context.Background()
	s, err := f.mp.CreateSession(ctx, f.hid, mealprep.SessionInput{Date: "2026-11-08", Name: "Niedziela"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := f.mp.AddDish(ctx, f.hid, s.ID, f.recipe.ID, portions)
	if err != nil {
		t.Fatal(err)
	}
	return d.ID
}

func prepIn(dish string, servings int) plan.EntryInput {
	return plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, Kind: domain.EntryPrep, PrepDishID: dish, Servings: servings}
}

func TestAddPrepEntryDefaultsToPortionsPerMeal(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.mp.SetPortionsPerMeal(ctx, f.hid, 3); err != nil {
		t.Fatal(err)
	}
	d := f.dish(t, 6)
	e, err := f.svc.Add(ctx, f.hid, wk, prepIn(d, 0))
	if err != nil || e.Servings != 3 || e.RecipeID != f.recipe.ID || e.Kind != domain.EntryPrep || e.PrepDishID != d {
		t.Fatalf("%v %+v", err, e)
	}
}

func TestAddPrepEntryOverRemainingInvalid(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, 4)
	if _, err := f.svc.Add(ctx, f.hid, wk, prepIn(d, 5)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
	if _, err := f.svc.Add(ctx, f.hid, wk, prepIn("", 1)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no source: %v", err)
	}
	if _, err := f.svc.Add(ctx, "h2", wk, prepIn(d, 1)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign dish: %v", err)
	}
	bare := plan.New(f.st, f.st)
	if _, err := bare.Add(ctx, f.hid, wk, prepIn(d, 1)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no portions: %v", err)
	}
}

func TestEditPrepEntryOwnPortionsAvailable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, 4)
	e, err := f.svc.Add(ctx, f.hid, wk, prepIn(d, 4))
	if err != nil {
		t.Fatal(err)
	}
	in := prepIn("", 4)
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); err != nil {
		t.Fatalf("same servings: %v", err)
	}
	in.Servings = 5
	if _, err := f.svc.Edit(ctx, f.hid, wk, e.ID, in); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("x5: %v", err)
	}
}

func TestEditCannotChangeKind(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, 4)
	e, _ := f.svc.Add(ctx, f.hid, wk, prepIn(d, 1))
	_, err := f.svc.Edit(ctx, f.hid, wk, e.ID, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, Kind: domain.EntryOut})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("kind: %v", err)
	}
	m, _ := f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, RecipeID: f.recipe.ID})
	if _, err := f.svc.Edit(ctx, f.hid, wk, m.ID, prepIn(d, 1)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("meal to prep: %v", err)
	}
	got, err := f.svc.Edit(ctx, f.hid, wk, e.ID, plan.EntryInput{Date: "2026-11-05", Slot: domain.SlotLunch, Servings: 2})
	if err != nil || got.Kind != domain.EntryPrep || got.PrepDishID != d || got.RecipeID != f.recipe.ID {
		t.Fatalf("carry: %v %+v", err, got)
	}
}

func TestAddOutEntry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	k := 800.0
	e, err := f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, Kind: domain.EntryOut, Servings: 5, OutValues: domain.Macros{Kcal: &k}})
	if err != nil || e.Servings != 1 || e.FreeText != "" || e.RecipeID != "" || e.OutValues.Kcal == nil || *e.OutValues.Kcal != 800 {
		t.Fatalf("%v %+v", err, e)
	}
	bad := 10001.0
	if _, err := f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, Kind: domain.EntryOut, OutValues: domain.Macros{Fat: &bad}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("range: %v", err)
	}
	week, _ := f.svc.Week(ctx, f.hid, wk)
	if len(week) != 1 || week[0].Kind != domain.EntryOut || *week[0].OutValues.Kcal != 800 {
		t.Fatalf("stored: %+v", week)
	}
}

func TestCopySkipsPrepEntries(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.dish(t, 4)
	f.svc.Add(ctx, f.hid, wk, prepIn(d, 1))
	f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotDinner, Kind: domain.EntryOut})
	f.svc.Add(ctx, f.hid, wk, plan.EntryInput{Date: "2026-11-04", Slot: domain.SlotLunch, RecipeID: f.recipe.ID})
	n, err := f.svc.Copy(ctx, f.hid, wk, "2026-11-09")
	if err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	got, _ := f.svc.Week(ctx, f.hid, "2026-11-09")
	for _, e := range got {
		if e.Kind == domain.EntryPrep {
			t.Fatalf("prep copied: %+v", e)
		}
	}
}

func TestPrepEntryDoneLeavesPantry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	h := &fakeHook{}
	f.svc.SetDoneHook(h)
	d := f.dish(t, 4)
	in := prepIn(d, 1)
	in.Done = true
	e, err := f.svc.Add(ctx, f.hid, wk, in)
	if err != nil || !e.Done || h.done != 1 {
		t.Fatalf("%v %+v done=%d", err, e, h.done)
	}
}
