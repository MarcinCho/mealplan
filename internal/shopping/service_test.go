package shopping_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/plan"
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
	return &fx{svc: shopping.New(st, st), plans: plan.New(st, st), rec: recipe.New(st, now), st: st}
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
