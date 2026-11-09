package pantry_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/pantry"
	"mealplan/internal/plan"
	"mealplan/internal/recipe"
)

func fp(v float64) *float64 { return &v }

var testProducts = []domain.Product{
	{ID: "p-makaron", Name: "makaron", NormalizedName: "makaron"},
	{ID: "p-oliwa", Name: "oliwa", NormalizedName: "oliwa"},
	{ID: "p-jajko", Name: "jajko", NormalizedName: "jajko", Aliases: []string{"jajka"}},
}

func testRecipe() domain.Recipe {
	return domain.Recipe{BaseServings: 4, Ingredients: []domain.Ingredient{
		{Name: "makaron", Quantity: fp(400), Unit: "g"},
		{Name: "oliwa", Quantity: fp(2), Unit: "łyżka"},
		{Name: "jajka", Quantity: fp(3), Unit: "szt"},
		{Name: "sól"},
	}}
}

func find(ms []domain.PantryMovement, product string) (domain.PantryMovement, bool) {
	for _, m := range ms {
		if m.ProductID == product {
			return m, true
		}
	}
	return domain.PantryMovement{}, false
}

func TestDeductionsScaleAndUnits(t *testing.T) {
	stock := []domain.PantryItem{
		{ProductID: "p-makaron", Quantity: fp(1000), Unit: "g"},
		{ProductID: "p-oliwa", Quantity: fp(500), Unit: "ml"},
		{ProductID: "p-jajko", Quantity: fp(6), Unit: "szt"},
	}
	ms := pantry.Deductions(testRecipe(), 2, nutrition.NewMatcher(testProducts), stock, "e1")
	if len(ms) != 2 {
		t.Fatalf("want 2 movements, got %+v", ms)
	}
	if m, _ := find(ms, "p-makaron"); m.Amount != -200 || m.Unit != "g" || m.EntryID != "e1" {
		t.Fatalf("makaron: %+v", m)
	}
	if m, _ := find(ms, "p-jajko"); m.Amount != -1.5 || m.Unit != "szt" {
		t.Fatalf("jajko: %+v", m)
	}
}

func TestDeductionsSkipStapleAndMissing(t *testing.T) {
	stock := []domain.PantryItem{
		{ProductID: "p-makaron", Unit: "g", Staple: true},
		{ProductID: "p-jajko", Quantity: fp(6), Unit: "szt"},
	}
	ms := pantry.Deductions(testRecipe(), 4, nutrition.NewMatcher(testProducts), stock, "e1")
	if len(ms) != 1 || ms[0].ProductID != "p-jajko" {
		t.Fatalf("got %+v", ms)
	}
}

func TestEntryDoneIsIdempotentAndUndoRestores(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	prod := f.product("Makaron testowy")
	r, err := recipe.New(f.st, nil).Create(ctx, f.hid, recipe.Input{Title: "Makaron", BaseServings: 2,
		Ingredients: []recipe.IngredientInput{{Name: "Makaron testowy", Quantity: fp(500), Unit: "g", ProductID: prod.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	psvc := plan.New(f.st, f.st)
	e, err := psvc.Add(ctx, f.hid, "2026-11-02", plan.EntryInput{Date: "2026-11-03", Slot: domain.SlotDinner, RecipeID: r.ID, Servings: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.AddToPantry(ctx, f.hid, prod.ID, "g", 300); err != nil {
		t.Fatal(err)
	}
	qty := func() float64 {
		items, _ := f.st.PantryItems(ctx, f.hid)
		for _, it := range items {
			if it.ProductID == prod.ID && it.Quantity != nil {
				return *it.Quantity
			}
		}
		return -1
	}
	step := func(name string, fn func() error, want float64) {
		t.Helper()
		if err := fn(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := qty(); got != want {
			t.Fatalf("%s: stock %v, want %v", name, got, want)
		}
	}
	done := func() error { return f.svc.EntryDone(ctx, f.hid, e) }
	undone := func() error { return f.svc.EntryUndone(ctx, f.hid, e.ID) }
	step("done", done, 0)
	step("done again", done, 0)
	step("undo", undone, 300)
	step("undo again", undone, 300)
	step("done after undo", done, 0)
}

func TestDeleteDoneEntryKeepsStock(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	prod := f.product("Ryż testowy")
	r, _ := recipe.New(f.st, nil).Create(ctx, f.hid, recipe.Input{Title: "Ryż", BaseServings: 1,
		Ingredients: []recipe.IngredientInput{{Name: "Ryż testowy", Quantity: fp(100), Unit: "g", ProductID: prod.ID}}})
	e, err := plan.New(f.st, f.st).Add(ctx, f.hid, "2026-11-02", plan.EntryInput{Date: "2026-11-03", Slot: domain.SlotDinner, RecipeID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.AddToPantry(ctx, f.hid, prod.ID, "g", 300); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.EntryDone(ctx, f.hid, e); err != nil {
		t.Fatal(err)
	}
	if err := f.st.DeletePlanEntry(ctx, f.hid, e.ID); err != nil {
		t.Fatal(err)
	}
	items, _ := f.st.PantryItems(ctx, f.hid)
	if len(items) != 1 || *items[0].Quantity != 200 {
		t.Fatalf("stock changed: %+v", items)
	}
	if ms, _ := f.st.MovementsForEntry(ctx, f.hid, e.ID); len(ms) != 0 {
		t.Fatalf("movements left: %+v", ms)
	}
}

// doneFixture returns a one-ingredient recipe entry (500 g for 2 servings) and the product.
func (f *fixture) doneFixture(t *testing.T, name string, servings int) (domain.PlanEntry, domain.Product) {
	t.Helper()
	ctx := context.Background()
	prod := f.product(name)
	r, err := recipe.New(f.st, nil).Create(ctx, f.hid, recipe.Input{Title: name, BaseServings: 2,
		Ingredients: []recipe.IngredientInput{{Name: name, Quantity: fp(500), Unit: "g", ProductID: prod.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	e, err := plan.New(f.st, f.st).Add(ctx, f.hid, "2026-11-02", plan.EntryInput{Date: "2026-11-03", Slot: domain.SlotDinner, RecipeID: r.ID, Servings: servings})
	if err != nil {
		t.Fatal(err)
	}
	return e, prod
}

func (f *fixture) stock(prodID string) float64 {
	items, _ := f.st.PantryItems(context.Background(), f.hid)
	for _, it := range items {
		if it.ProductID == prodID && it.Quantity != nil {
			return *it.Quantity
		}
	}
	return -1
}

func TestConcurrentDoneAndUndoApplyOnce(t *testing.T) {
	if os.Getenv("MEALPLAN_TEST_DRIVER") != "postgres" {
		t.Skip("Postgres-only concurrency test")
	}
	f := newFixture(t)
	ctx := context.Background()
	e, prod := f.doneFixture(t, "Wspólny", 2)
	if _, err := f.st.AddToPantry(ctx, f.hid, prod.ID, "g", 2000); err != nil {
		t.Fatal(err)
	}
	run := func(fn func() error) {
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	}
	run(func() error { return f.svc.EntryDone(ctx, f.hid, e) })
	if got := f.stock(prod.ID); got != 1500 {
		t.Fatalf("after parallel done: %v", got)
	}
	if ms, _ := f.st.MovementsForEntry(ctx, f.hid, e.ID); len(ms) != 1 {
		t.Fatalf("movements: %+v", ms)
	}
	run(func() error { return f.svc.EntryUndone(ctx, f.hid, e.ID) })
	if got := f.stock(prod.ID); got != 2000 {
		t.Fatalf("after parallel undo: %v", got)
	}
}

type failingRepo struct {
	pantry.Repo
	fail bool
}

func (r *failingRepo) ConsumeForEntry(ctx context.Context, hh, id string, ms []domain.PantryMovement) (bool, error) {
	if r.fail {
		r.fail = false
		return false, errors.New("db down")
	}
	return r.Repo.ConsumeForEntry(ctx, hh, id, ms)
}

func TestRetryAfterFailingStoreDeductsOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	e, prod := f.doneFixture(t, "Retry", 2)
	f.st.AddToPantry(ctx, f.hid, prod.ID, "g", 2000)
	svc := pantry.New(&failingRepo{Repo: f.st, fail: true}, f.st, f.prods, nil)
	psvc := plan.New(f.st, f.st)
	psvc.SetDoneHook(svc)
	in := plan.EntryInput{Date: e.Date, Slot: e.Slot, RecipeID: e.RecipeID, Servings: 2, Done: true}
	if _, err := psvc.Edit(ctx, f.hid, "2026-11-02", e.ID, in); !errors.Is(err, plan.ErrPantry) {
		t.Fatalf("first: %v", err)
	}
	if got := f.stock(prod.ID); got != 2000 {
		t.Fatalf("stock after failure: %v", got)
	}
	for i := 0; i < 2; i++ { // retry, then a second done=true save
		if _, err := psvc.Edit(ctx, f.hid, "2026-11-02", e.ID, in); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.stock(prod.ID); got != 1500 {
		t.Fatalf("stock after retries: %v", got)
	}
}

func TestAddWithDoneDeducts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	e, prod := f.doneFixture(t, "Dodane", 2)
	f.st.AddToPantry(ctx, f.hid, prod.ID, "g", 2000)
	psvc := plan.New(f.st, f.st)
	psvc.SetDoneHook(f.svc)
	if _, err := psvc.Add(ctx, f.hid, "2026-11-02", plan.EntryInput{Date: e.Date, Slot: domain.SlotLunch, RecipeID: e.RecipeID, Servings: 2, Done: true}); err != nil {
		t.Fatal(err)
	}
	if got := f.stock(prod.ID); got != 1500 {
		t.Fatalf("stock: %v", got)
	}
}

func TestStapleAndStockInKgShareOneRow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := f.product("Mąka kg")
	if _, err := f.svc.Add(ctx, f.hid, pantry.Input{ProductID: p.ID, Unit: "kg", Staple: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, f.hid, pantry.Input{ProductID: p.ID, Quantity: fp(1), Unit: "kg"}); err != nil {
		t.Fatal(err)
	}
	items, _ := f.st.PantryItems(ctx, f.hid)
	if len(items) != 1 || items[0].Unit != "g" || !items[0].Staple {
		t.Fatalf("items: %+v", items)
	}
}

func TestDeductionsUnitlessIngredientCountsAsPieces(t *testing.T) {
	r := domain.Recipe{BaseServings: 2, Ingredients: []domain.Ingredient{{Name: "jajka", Quantity: fp(3)}}}
	stock := []domain.PantryItem{{ProductID: "p-jajko", Quantity: fp(6), Unit: "szt"}}
	ms := pantry.Deductions(r, 2, nutrition.NewMatcher(testProducts), stock, "e1")
	if len(ms) != 1 || ms[0].Unit != "szt" || ms[0].Amount != -3 {
		t.Fatalf("want -3 szt, got %+v", ms)
	}
}

func TestEntryDoneSkipsPrepEntries(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	prod := f.product("Kasza testowa")
	r, err := recipe.New(f.st, nil).Create(ctx, f.hid, recipe.Input{Title: "Kasza", BaseServings: 1,
		Ingredients: []recipe.IngredientInput{{Name: "Kasza testowa", Quantity: fp(100), Unit: "g", ProductID: prod.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.AddToPantry(ctx, f.hid, prod.ID, "g", 300); err != nil {
		t.Fatal(err)
	}
	e := domain.PlanEntry{ID: "e1", HouseholdID: f.hid, RecipeID: r.ID, Servings: 1, Kind: domain.EntryPrep, PrepDishID: "d1"}
	if err := f.svc.EntryDone(ctx, f.hid, e); err != nil {
		t.Fatal(err)
	}
	items, _ := f.st.PantryItems(ctx, f.hid)
	for _, it := range items {
		if it.ProductID == prod.ID && (it.Quantity == nil || *it.Quantity != 300) {
			t.Fatalf("stock changed: %v", *it.Quantity)
		}
	}
}
