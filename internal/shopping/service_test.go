package shopping_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/mealprep"
	"mealplan/internal/pantry"
	"mealplan/internal/plan"
	"mealplan/internal/product"
	"mealplan/internal/recipe"
	"mealplan/internal/shopping"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

type fx struct {
	svc   *shopping.Service
	plans *plan.Service
	rec   *recipe.Service
	st    *store.Store
	pan   *pantry.Service
	mp    *mealprep.Service
}

const hid = "h1"
const wk = "2026-11-02"

func newFx(t *testing.T) *fx {
	t.Helper()
	st := store.New(storetest.Open(t))
	ctx := context.Background()
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: hid, Name: "Dom"}, domain.User{ID: "u1", Login: "marc", PasswordHash: "x", DisplayName: "M"}, false); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: "h2", Name: "Inny"}, domain.User{ID: "u2", Login: "eve", PasswordHash: "x", DisplayName: "E"}, false); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
	prods := product.New(st, now)
	pan := pantry.New(st, st, prods, now)
	svc := shopping.New(st, st, pan, prods)
	svc.SetSessions(st)
	mp := mealprep.New(st, st, nil, now)
	ps := plan.New(st, st)
	ps.SetPortions(mp)
	return &fx{svc: svc, plans: ps, rec: recipe.New(st, now), st: st, pan: pan, mp: mp}
}

func (f *fx) cook(t *testing.T, title string, servings int, ings ...recipe.IngredientInput) string {
	t.Helper()
	r, err := f.rec.Create(context.Background(), hid, recipe.Input{Title: title, BaseServings: servings, Ingredients: ings})
	if err != nil {
		t.Fatal(err)
	}
	return r.ID
}

func (f *fx) plan(t *testing.T, date, recipeID string, servings int) {
	t.Helper()
	if _, err := f.plans.Add(context.Background(), hid, wk, plan.EntryInput{Date: date, Slot: domain.SlotDinner, RecipeID: recipeID, Servings: servings}); err != nil {
		t.Fatal(err)
	}
}

func byName(items []domain.ShoppingItem, name string) *domain.ShoppingItem {
	for i := range items {
		if items[i].NormalizedName == name {
			return &items[i]
		}
	}
	return nil
}

func TestGenerateFromPlan(t *testing.T) {
	f := newFx(t)
	r := f.cook(t, "Naleśniki", 4, recipe.IngredientInput{Name: "Mąka", Quantity: q(200), Unit: "g"}, recipe.IngredientInput{Name: "Sól"})
	f.plan(t, "2026-11-03", r, 8)
	items, err := f.svc.Generate(context.Background(), hid, wk, "", "")
	if err != nil || len(items) != 2 {
		t.Fatalf("generate: %v %+v", err, items)
	}
	flour := byName(items, "mąka")
	if flour == nil || *flour.Quantity != 400 || flour.Unit != "g" || flour.Manual || flour.Checked || flour.Week != wk {
		t.Fatalf("flour: %+v", flour)
	}
	if salt := byName(items, "sól"); salt == nil || salt.Quantity != nil {
		t.Fatalf("salt: %+v", salt)
	}
}

func TestGenerateDayRange(t *testing.T) {
	f := newFx(t)
	a := f.cook(t, "A", 1, recipe.IngredientInput{Name: "Ser", Quantity: q(100), Unit: "g"})
	b := f.cook(t, "B", 1, recipe.IngredientInput{Name: "Szynka", Quantity: q(50), Unit: "g"})
	f.plan(t, "2026-11-03", a, 1)
	f.plan(t, "2026-11-06", b, 1)
	items, err := f.svc.Generate(context.Background(), hid, wk, "2026-11-05", "2026-11-08")
	if err != nil || len(items) != 1 || items[0].NormalizedName != "szynka" {
		t.Fatalf("range: %v %+v", err, items)
	}
	if _, err := f.svc.Generate(context.Background(), hid, wk, "2026-11-10", "2026-11-12"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("range outside week: %v", err)
	}
	if _, err := f.svc.Generate(context.Background(), hid, wk, "2026-11-06", "2026-11-05"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("reversed range: %v", err)
	}
}

func TestRegenerationKeepsManualAndCheckedState(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 2, recipe.IngredientInput{Name: "Marchew", Quantity: q(2), Unit: "szt"}, recipe.IngredientInput{Name: "Pory", Quantity: q(1), Unit: "szt"})
	f.plan(t, "2026-11-03", r, 2)
	items, _ := f.svc.Generate(ctx, hid, wk, "", "")
	manual, err := f.svc.AddManual(ctx, hid, wk, shopping.ItemInput{Name: "Papier toaletowy"})
	if err != nil || !manual.Manual {
		t.Fatalf("manual: %v %+v", err, manual)
	}
	carrotID := byName(items, "marchew").ID
	if err := f.svc.SetChecked(ctx, hid, carrotID, true); err != nil {
		t.Fatal(err)
	}
	leekID := byName(items, "pory").ID
	if err := f.svc.SetChecked(ctx, hid, leekID, true); err != nil {
		t.Fatal(err)
	}

	// Change the plan: leeks double, carrots stay the same.
	f.plan(t, "2026-11-04", f.cook(t, "Sałatka", 1, recipe.IngredientInput{Name: "Pory", Quantity: q(1), Unit: "szt"}), 1)
	again, err := f.svc.Generate(ctx, hid, wk, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if c := byName(again, "marchew"); c == nil || c.ID != carrotID || !c.Checked {
		t.Fatalf("unchanged item must keep id and checked: %+v", c)
	}
	if l := byName(again, "pory"); l == nil || l.ID != leekID || *l.Quantity != 2 || l.Checked {
		t.Fatalf("changed item must reset checked: %+v", l)
	}
	if m := byName(again, "papier toaletowy"); m == nil || m.ID != manual.ID || !m.Manual {
		t.Fatalf("manual item must survive: %+v", m)
	}
}

func TestRegenerationDropsItemsNoLongerNeeded(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Marchew", Quantity: q(1), Unit: "szt"})
	f.plan(t, "2026-11-03", r, 1)
	_, _ = f.svc.Generate(ctx, hid, wk, "", "")
	entries, _ := f.plans.Week(ctx, hid, wk)
	_ = f.plans.Remove(ctx, hid, wk, entries[0].ID)
	items, err := f.svc.Generate(ctx, hid, wk, "", "")
	if err != nil || len(items) != 0 {
		t.Fatalf("stale item must go: %v %+v", err, items)
	}
}

func TestManualItemsAndEditing(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	m, _ := f.svc.AddManual(ctx, hid, wk, shopping.ItemInput{Name: "  Chleb ", Quantity: q(2), Unit: "Szt."})
	if m.Name != "Chleb" || m.NormalizedName != "chleb" || m.Unit != "szt" {
		t.Fatalf("normalized: %+v", m)
	}
	ed, err := f.svc.Edit(ctx, hid, m.ID, shopping.ItemInput{Name: "Bułki", Quantity: q(6), Unit: "szt"})
	if err != nil || ed.Name != "Bułki" || *ed.Quantity != 6 {
		t.Fatalf("edit: %v %+v", err, ed)
	}
	for name, in := range map[string]shopping.ItemInput{
		"empty name":    {Name: " "},
		"zero quantity": {Name: "x", Quantity: q(0)},
		"huge quantity": {Name: "x", Quantity: q(1e9)},
		"nul in name":   {Name: "mle\x00ko"},
		"nul in unit":   {Name: "x", Unit: "k\x00g"},
		"bell in name":  {Name: "mle\x07ko"},
	} {
		if _, err := f.svc.AddManual(ctx, hid, wk, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := f.svc.AddManual(ctx, hid, "2026-11-03", shopping.ItemInput{Name: "x"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("non-monday week: %v", err)
	}
	if err := f.svc.Remove(ctx, "h2", m.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("remove foreign: %v", err)
	}
	if err := f.svc.Remove(ctx, hid, m.ID); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedItemsCannotBeRenamed(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Marchew", Quantity: q(1), Unit: "szt"})
	f.plan(t, "2026-11-03", r, 1)
	items, _ := f.svc.Generate(ctx, hid, wk, "", "")
	if _, err := f.svc.Edit(ctx, hid, items[0].ID, shopping.ItemInput{Name: "Inne"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("generated item edit: %v", err)
	}
	if err := f.svc.SetChecked(ctx, hid, items[0].ID, true); err != nil {
		t.Fatalf("checking a generated item is allowed: %v", err)
	}
}

func TestGenerateIgnoresForeignHouseholdData(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Marchew", Quantity: q(1), Unit: "szt"})
	f.plan(t, "2026-11-03", r, 1)
	items, err := f.svc.Generate(ctx, "h2", wk, "", "")
	if err != nil || len(items) != 0 {
		t.Fatalf("other household must get an empty list: %v %+v", err, items)
	}
}

func TestGenerateKeepsCoveredRowsAndCheckedState(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Naleśniki", 1, recipe.IngredientInput{Name: "Mąka", Quantity: q(500), Unit: "g"}, recipe.IngredientInput{Name: "Mleko", Quantity: q(300), Unit: "ml"})
	f.plan(t, "2026-11-03", r, 1)
	if _, err := f.pan.Add(ctx, hid, pantry.Input{Name: "Mąka", Quantity: q(2), Unit: "kg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pan.Add(ctx, hid, pantry.Input{Name: "Mleko", Quantity: q(100), Unit: "ml"}); err != nil {
		t.Fatal(err)
	}
	items, err := f.svc.Generate(ctx, hid, wk, "", "")
	if err != nil || len(items) != 2 {
		t.Fatalf("generate: %v %+v", err, items)
	}
	flour := byName(items, "mąka")
	if flour == nil || flour.Quantity == nil || *flour.Quantity != 0 || flour.HaveQuantity == nil || *flour.HaveQuantity != 500 || flour.Unit != "g" {
		t.Fatalf("covered flour must stay with have: %+v", flour)
	}
	milk := byName(items, "mleko")
	if milk == nil || *milk.Quantity != 200 || milk.HaveQuantity == nil || *milk.HaveQuantity != 100 {
		t.Fatalf("partial milk: %+v", milk)
	}
	if err := f.svc.SetChecked(ctx, hid, flour.ID, true); err != nil {
		t.Fatal(err)
	}
	again, _ := f.svc.Generate(ctx, hid, wk, "", "")
	if c := byName(again, "mąka"); c == nil || c.ID != flour.ID || !c.Checked {
		t.Fatalf("covered row must keep id and checked: %+v", c)
	}
}

func TestGenerateSkipsDoneAndStaple(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Sól"}, recipe.IngredientInput{Name: "Marchew", Quantity: q(2), Unit: "szt"})
	f.plan(t, "2026-11-03", r, 1)
	if _, err := f.pan.Add(ctx, hid, pantry.Input{Name: "Sól", Staple: true, Unit: "szt"}); err != nil {
		t.Fatal(err)
	}
	items, err := f.svc.Generate(ctx, hid, wk, "", "")
	if err != nil || len(items) != 1 || items[0].NormalizedName != "marchew" {
		t.Fatalf("staple must be skipped: %v %+v", err, items)
	}
}

func TestMoveCheckedToPantryOnce(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Marchew", Quantity: q(2), Unit: "szt"}, recipe.IngredientInput{Name: "Pory", Quantity: q(1), Unit: "szt"})
	f.plan(t, "2026-11-03", r, 1)
	items, _ := f.svc.Generate(ctx, hid, wk, "", "")
	man, err := f.svc.AddManual(ctx, hid, wk, shopping.ItemInput{Name: "Kasza bulgur", Quantity: q(0.5), Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{man.ID, byName(items, "marchew").ID} {
		if err := f.svc.SetChecked(ctx, hid, id, true); err != nil {
			t.Fatal(err)
		}
	}
	n, err := f.svc.MoveCheckedToPantry(ctx, hid, wk)
	if err != nil || n != 2 {
		t.Fatalf("move: %d %v", n, err)
	}
	stock := func() map[string]float64 {
		list, _, err := f.pan.List(ctx, hid)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]float64{}
		for _, it := range list {
			m[it.Name+"|"+it.Unit] = *it.Quantity
		}
		return m
	}
	before := stock()
	if len(before) != 2 {
		t.Fatalf("pantry: %+v", before)
	}
	var bulgur float64
	for k, v := range before {
		if strings.Contains(strings.ToLower(k), "bulgur") && strings.HasSuffix(k, "|g") {
			bulgur = v
		}
	}
	if bulgur != 500 {
		t.Fatalf("bulgur must be 500 g: %+v", before)
	}
	n, err = f.svc.MoveCheckedToPantry(ctx, hid, wk)
	if err != nil || n != 0 {
		t.Fatalf("second move: %d %v", n, err)
	}
	after := stock()
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("stock changed: %+v -> %+v", before, after)
		}
	}
}

func TestMoveCheckedSkipsCoveredAndNoQuantity(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Mąka", Quantity: q(100), Unit: "g"}, recipe.IngredientInput{Name: "Pieprz"})
	f.plan(t, "2026-11-03", r, 1)
	if _, err := f.pan.Add(ctx, hid, pantry.Input{Name: "Mąka", Quantity: q(1), Unit: "kg"}); err != nil {
		t.Fatal(err)
	}
	items, _ := f.svc.Generate(ctx, hid, wk, "", "")
	for _, it := range items {
		if err := f.svc.SetChecked(ctx, hid, it.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := f.svc.MoveCheckedToPantry(ctx, hid, wk); err != nil || n != 0 {
		t.Fatalf("covered/no-quantity must not move: %d %v", n, err)
	}
}

type failingPantry struct {
	shopping.Pantry
	failName string
	fail     bool
}

func (p *failingPantry) Add(ctx context.Context, hid string, in pantry.Input) (domain.PantryItem, error) {
	if p.fail && in.Name == p.failName {
		return domain.PantryItem{}, errors.New("boom")
	}
	return p.Pantry.Add(ctx, hid, in)
}

func TestMoveCheckedFailureReleasesOnlyFailedItem(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	fp := &failingPantry{Pantry: f.pan, failName: "Pory", fail: true}
	prods := product.New(f.st, nil)
	svc := shopping.New(f.st, f.st, fp, prods)
	var ids []string
	for _, n := range []string{"Marchew", "Pory", "Seler"} {
		it, err := svc.AddManual(ctx, hid, wk, shopping.ItemInput{Name: n, Quantity: q(1), Unit: "szt"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, it.ID)
		if err := svc.SetChecked(ctx, hid, it.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	n, err := svc.MoveCheckedToPantry(ctx, hid, wk)
	if err == nil || n != 2 {
		t.Fatalf("partial failure: %d %v", n, err)
	}
	items, _ := svc.List(ctx, hid, wk)
	for _, it := range items {
		if it.Moved != (it.NormalizedName != "pory") {
			t.Fatalf("moved flags: %+v", items)
		}
	}
	fp.fail = false
	if n, err := svc.MoveCheckedToPantry(ctx, hid, wk); err != nil || n != 1 {
		t.Fatalf("retry: %d %v", n, err)
	}
	list, _, _ := f.pan.List(ctx, hid)
	if len(list) != 3 {
		t.Fatalf("each item once: %+v", list)
	}
	for _, it := range list {
		if *it.Quantity != 1 {
			t.Fatalf("doubled: %+v", list)
		}
	}
}

func TestMoveCheckedParallelAddsOnce(t *testing.T) {
	if os.Getenv("MEALPLAN_TEST_DRIVER") != "postgres" {
		t.Skip("parallel claim test needs Postgres")
	}
	f := newFx(t)
	ctx := context.Background()
	for _, n := range []string{"Marchew", "Pory", "Seler"} {
		it, _ := f.svc.AddManual(ctx, hid, wk, shopping.ItemInput{Name: n, Quantity: q(1), Unit: "szt"})
		_ = f.svc.SetChecked(ctx, hid, it.ID, true)
	}
	var wg sync.WaitGroup
	var total atomic.Int64
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, _ := f.svc.MoveCheckedToPantry(ctx, hid, wk)
			total.Add(int64(n))
		}()
	}
	wg.Wait()
	list, _, _ := f.pan.List(ctx, hid)
	if total.Load() != 3 || len(list) != 3 {
		t.Fatalf("total %d, pantry %+v", total.Load(), list)
	}
	for _, it := range list {
		if *it.Quantity != 1 {
			t.Fatalf("doubled: %+v", list)
		}
	}
}

func (f *fx) planIn(t *testing.T, week, date string, in plan.EntryInput) {
	t.Helper()
	in.Date, in.Slot = date, domain.SlotDinner
	if _, err := f.plans.Add(context.Background(), hid, week, in); err != nil {
		t.Fatal(err)
	}
}

func TestPreviousWeekUndoneCountsOnlyNotDoneRecipeEntries(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Zupa", 1, recipe.IngredientInput{Name: "Marchew", Quantity: q(2), Unit: "szt"})
	prev := "2026-10-26"
	if n, err := f.svc.PreviousWeekUndone(ctx, hid, wk); err != nil || n != 0 {
		t.Fatalf("empty: %d %v", n, err)
	}
	f.planIn(t, prev, "2026-10-26", plan.EntryInput{RecipeID: r})
	f.planIn(t, prev, "2026-11-01", plan.EntryInput{RecipeID: r})
	f.planIn(t, prev, "2026-10-28", plan.EntryInput{RecipeID: r, Done: true})
	f.planIn(t, prev, "2026-10-29", plan.EntryInput{FreeText: "pizza"})
	f.planIn(t, wk, "2026-11-03", plan.EntryInput{RecipeID: r}) // current week does not count
	n, err := f.svc.PreviousWeekUndone(ctx, hid, wk)
	if err != nil || n != 2 {
		t.Fatalf("want 2, got %d %v", n, err)
	}
	if _, err := f.svc.PreviousWeekUndone(ctx, hid, "bad"); err == nil {
		t.Fatal("invalid week must fail")
	}
}

func TestMoveCheckedUsesLinkedProduct(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	now := func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
	prods := product.New(f.st, now)
	p, err := prods.Create(ctx, hid, product.Input{Name: "mąka amarantusowa"})
	if err != nil {
		t.Fatal(err)
	}
	r := f.cook(t, "Placek", 1, recipe.IngredientInput{Name: "mąki", Quantity: q(500), Unit: "g", ProductID: p.ID})
	f.plan(t, "2026-11-03", r, 1)
	items, err := f.svc.Generate(ctx, hid, wk, "", "")
	if err != nil {
		t.Fatal(err)
	}
	it := byName(items, "mąki")
	if it == nil {
		t.Fatalf("items: %+v", items)
	}
	if err := f.svc.SetChecked(ctx, hid, it.ID, true); err != nil {
		t.Fatal(err)
	}
	if n, err := f.svc.MoveCheckedToPantry(ctx, hid, wk); err != nil || n != 1 {
		t.Fatalf("move: %d %v", n, err)
	}
	list, _, err := f.pan.List(ctx, hid)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ProductID != p.ID || *list[0].Quantity != 500 {
		t.Fatalf("pantry: %+v", list)
	}
	all, _ := prods.All(ctx, hid)
	for _, pr := range all {
		if pr.NormalizedName == "mąki" {
			t.Fatalf("stray product created: %+v", pr)
		}
	}
}

func TestGenerateIncludesSessionOfWeek(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	r := f.cook(t, "Ryż z warzywami", 4, recipe.IngredientInput{Name: "Ryż", Quantity: q(200), Unit: "g"})
	sess, err := f.mp.CreateSession(ctx, hid, mealprep.SessionInput{Date: "2026-11-08", Name: "Niedziela"})
	if err != nil || sess.Week != "2026-11-09" {
		t.Fatalf("%v %+v", err, sess)
	}
	dish, err := f.mp.AddDish(ctx, hid, sess.ID, r, 12)
	if err != nil {
		t.Fatal(err)
	}
	next := "2026-11-09"
	items, err := f.svc.Generate(ctx, hid, next, "2026-11-11", "2026-11-13")
	rice := byName(items, "ryż")
	if err != nil || rice == nil || *rice.Quantity != 600 {
		t.Fatalf("%v %+v", err, items)
	}
	if items, err = f.svc.Generate(ctx, hid, wk, "", ""); err != nil || len(items) != 0 {
		t.Fatalf("other week: %v %+v", err, items)
	}
	// an allocated prep entry adds nothing
	if _, err := f.plans.Add(ctx, hid, next, plan.EntryInput{Date: "2026-11-10", Slot: domain.SlotDinner, Kind: domain.EntryPrep, PrepDishID: dish.ID, Servings: 2}); err != nil {
		t.Fatal(err)
	}
	if items, _ = f.svc.Generate(ctx, hid, next, "", ""); byName(items, "ryż") == nil || *byName(items, "ryż").Quantity != 600 {
		t.Fatalf("prep entry changed list: %+v", items)
	}
	if n, err := f.svc.PreviousWeekUndone(ctx, hid, "2026-11-16"); err != nil || n != 0 {
		t.Fatalf("prep entry counted: %d %v", n, err)
	}
	// cooked session: ingredients were already used
	if _, err := f.mp.UpdateSession(ctx, hid, sess.ID, mealprep.SessionInput{Date: "2026-11-08", Name: "Niedziela", Cooked: true}); err != nil {
		t.Fatal(err)
	}
	if items, _ = f.svc.Generate(ctx, hid, next, "", ""); byName(items, "ryż") != nil {
		t.Fatalf("cooked session still listed: %+v", items)
	}
}
