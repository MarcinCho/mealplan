package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func item(id, hid, week, name string, manual bool) domain.ShoppingItem {
	return domain.ShoppingItem{ID: id, HouseholdID: hid, Week: week, Name: name, NormalizedName: name, Quantity: f(1), Unit: "szt", Manual: manual}
}

func TestShoppingItemsOrderAndIsolation(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	b := item("i2", "hA", "2026-11-02", "banan", false)
	a := item("i1", "hA", "2026-11-02", "ananas", false)
	a.Checked = true
	for _, it := range []domain.ShoppingItem{a, b, item("iB", "hB", "2026-11-02", "tajne", false), item("iW", "hA", "2026-11-09", "inny tydzień", false)} {
		if err := s.InsertShoppingItem(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ShoppingItems(ctx, "hA", "2026-11-02")
	if err != nil || len(got) != 2 || got[0].ID != "i2" || got[1].ID != "i1" {
		t.Fatalf("unchecked first: %v %+v", err, got)
	}
	if !got[1].Checked || got[1].Quantity == nil || *got[1].Quantity != 1 {
		t.Fatalf("fields: %+v", got[1])
	}
	if err := s.UpdateShoppingItem(ctx, item("i1", "hB", "2026-11-02", "hacked", false)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update foreign: %v", err)
	}
	if err := s.DeleteShoppingItem(ctx, "hB", "i1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete foreign: %v", err)
	}
	if _, err := s.ShoppingItemByID(ctx, "hB", "i1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get foreign: %v", err)
	}
}

func TestReplaceGeneratedKeepsManualItems(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.InsertShoppingItem(ctx, item("g1", "hA", "2026-11-02", "stare", false))
	_ = s.InsertShoppingItem(ctx, item("m1", "hA", "2026-11-02", "papier", true))
	_ = s.InsertShoppingItem(ctx, item("gB", "hB", "2026-11-02", "obce", false))
	if err := s.ReplaceGeneratedItems(ctx, "hA", "2026-11-02", []domain.ShoppingItem{item("g2", "hA", "2026-11-02", "nowe", false)}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ShoppingItems(ctx, "hA", "2026-11-02")
	if len(got) != 2 {
		t.Fatalf("after replace: %+v", got)
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids["m1"] || !ids["g2"] || ids["g1"] {
		t.Fatalf("wrong survivors: %+v", got)
	}
	if other, _ := s.ShoppingItems(ctx, "hB", "2026-11-02"); len(other) != 1 {
		t.Fatalf("other household touched: %+v", other)
	}
}

func TestRecipesByIDsOnlyReturnsOwnRecipes(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("rA", "hA"))
	_ = s.CreateRecipe(ctx, sampleRecipe("rB", "hB"))
	got, err := s.RecipesByIDs(ctx, "hA", []string{"rA", "rB", "missing"})
	if err != nil || len(got) != 1 || got["rA"].Title != "Naleśniki" || len(got["rA"].Ingredients) != 2 {
		t.Fatalf("recipes: %v %+v", err, got)
	}
	if empty, err := s.RecipesByIDs(ctx, "hA", nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty: %v %+v", err, empty)
	}
}

func TestShoppingItemHaveAndMovedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	a := item("i1", "hA", "2026-11-02", "mąka", false)
	a.HaveQuantity = f(200)
	b := item("i2", "hA", "2026-11-02", "sól", false)
	c := item("i3", "hB", "2026-11-02", "obce", false)
	for _, it := range []domain.ShoppingItem{a, b, c} {
		if err := s.InsertShoppingItem(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.ShoppingItemByID(ctx, "hA", "i1")
	if got.HaveQuantity == nil || *got.HaveQuantity != 200 || got.Moved {
		t.Fatalf("round trip: %+v", got)
	}
	if err := s.SetShoppingItemsMoved(ctx, "hA", []string{"i1", "i3"}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ShoppingItemByID(ctx, "hA", "i1")
	other, _ := s.ShoppingItemByID(ctx, "hA", "i2")
	foreign, _ := s.ShoppingItemByID(ctx, "hB", "i3")
	if !got.Moved || other.Moved || foreign.Moved || other.HaveQuantity != nil {
		t.Fatalf("moved: %+v %+v %+v", got, other, foreign)
	}
	if err := s.ReplaceGeneratedItems(ctx, "hA", "2026-11-02", []domain.ShoppingItem{a}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ShoppingItems(ctx, "hA", "2026-11-02")
	if len(list) != 1 || list[0].HaveQuantity == nil || *list[0].HaveQuantity != 200 {
		t.Fatalf("replace: %+v", list)
	}
}
