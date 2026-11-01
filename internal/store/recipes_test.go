package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func seedTwoHouseholds(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()
	if err := s.CreateHouseholdWithAdmin(ctx, household("hA"), user("uA", "alice"), false); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateHouseholdWithAdmin(ctx, household("hB"), user("uB", "bob"), false); err != nil {
		t.Fatal(err)
	}
}

func f(v float64) *float64 { return &v }

func sampleRecipe(id, hid string) domain.Recipe {
	return domain.Recipe{
		ID: id, HouseholdID: hid, Title: "Naleśniki", BaseServings: 4, SourceURL: "https://example.com/n",
		Tags:        []string{"deser", "szybkie"},
		Ingredients: []domain.Ingredient{{Name: "Mąka", NormalizedName: "mąka", Quantity: f(250), Unit: "g"}, {Name: "Sól", NormalizedName: "sól"}},
		Steps:       []string{"Wymieszaj.", "Smaż."},
		CreatedAt:   now, UpdatedAt: now,
	}
}

func TestRecipeRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	if err := s.CreateRecipe(ctx, sampleRecipe("r1", "hA")); err != nil {
		t.Fatal(err)
	}
	got, err := s.RecipeByID(ctx, "hA", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Naleśniki" || got.BaseServings != 4 || len(got.Ingredients) != 2 || len(got.Steps) != 2 || len(got.Tags) != 2 {
		t.Fatalf("round trip: %+v", got)
	}
	if got.Ingredients[0].Quantity == nil || *got.Ingredients[0].Quantity != 250 || got.Ingredients[1].Quantity != nil {
		t.Fatalf("quantities: %+v", got.Ingredients)
	}
	if got.Ingredients[0].Name != "Mąka" || got.Steps[1] != "Smaż." {
		t.Fatalf("order lost: %+v", got)
	}
}

func TestRecipeUpdateReplacesChildren(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	upd := sampleRecipe("r1", "hA")
	upd.Title = "Racuchy"
	upd.Tags = []string{"obiad"}
	upd.Ingredients = upd.Ingredients[:1]
	upd.Steps = nil
	if err := s.UpdateRecipe(ctx, upd); err != nil {
		t.Fatal(err)
	}
	got, _ := s.RecipeByID(ctx, "hA", "r1")
	if got.Title != "Racuchy" || len(got.Tags) != 1 || len(got.Ingredients) != 1 || len(got.Steps) != 0 {
		t.Fatalf("after update: %+v", got)
	}
}

func TestRecipeHouseholdIsolation(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))

	if _, err := s.RecipeByID(ctx, "hB", "r1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("read across households: %v", err)
	}
	other := sampleRecipe("r1", "hB")
	other.Title = "Hacked"
	if err := s.UpdateRecipe(ctx, other); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update across households: %v", err)
	}
	if err := s.DeleteRecipe(ctx, "hB", "r1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete across households: %v", err)
	}
	if list, _ := s.ListRecipes(ctx, "hB"); len(list) != 0 {
		t.Fatalf("list leaks: %+v", list)
	}
	got, _ := s.RecipeByID(ctx, "hA", "r1")
	if got.Title != "Naleśniki" || len(got.Ingredients) != 2 {
		t.Fatalf("victim recipe changed: %+v", got)
	}
}

func TestListRecipesIncludesTagsWithoutBodies(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	list, err := s.ListRecipes(ctx, "hA")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if len(list[0].Tags) != 2 || len(list[0].Ingredients) != 0 || len(list[0].Steps) != 0 {
		t.Fatalf("summary shape: %+v", list[0])
	}
}

func TestDeleteRecipeInUseIsRefused(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO plan_entry (id, household_id, date, slot, recipe_id, free_text, servings) VALUES ('e1','hA','2026-11-02','dinner','r1','',2)`); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRecipe(ctx, "hA", "r1"); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("expected ErrInUse, got %v", err)
	}
	if _, err := s.RecipeByID(ctx, "hA", "r1"); err != nil {
		t.Fatalf("recipe must survive: %v", err)
	}
}

func TestDeleteRecipeRemovesChildren(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	_ = s.CreateRecipe(ctx, sampleRecipe("r1", "hA"))
	if err := s.DeleteRecipe(ctx, "hA", "r1"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM ingredient)+(SELECT COUNT(*) FROM step)+(SELECT COUNT(*) FROM recipe_tag)`).Scan(&n)
	if n != 0 {
		t.Fatalf("orphans left: %d", n)
	}
}
