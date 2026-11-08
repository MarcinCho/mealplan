package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func prod(id, hid, name string, aliases ...string) domain.Product {
	return domain.Product{ID: id, HouseholdID: hid, Name: name, NormalizedName: name,
		Per100g: domain.Macros{Kcal: f(100), Protein: f(5)}, PieceGrams: f(50), Aliases: aliases, CreatedAt: now, UpdatedAt: now}
}

func TestProductCRUDAndAliases(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	if err := s.CreateProduct(ctx, prod("p1", "hA", "mleko", "mleczko", "mleko2")); err != nil {
		t.Fatal(err)
	}
	got, err := s.ProductByID(ctx, "hA", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Per100g.Kcal == nil || *got.Per100g.Kcal != 100 || got.Per100g.Fat != nil || got.DensityGPerML != nil || *got.PieceGrams != 50 || len(got.Aliases) != 2 {
		t.Fatalf("got %+v", got)
	}
	if err := s.CreateProduct(ctx, prod("p2", "hA", "mleko")); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("dup name: %v", err)
	}
	if err := s.CreateProduct(ctx, prod("p3", "hA", "ser", "mleczko")); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("dup alias: %v", err)
	}
	if err := s.CreateProduct(ctx, prod("p4", "hB", "mleko", "mleczko")); err != nil {
		t.Fatalf("other household: %v", err)
	}
	up := prod("p1", "hA", "mleko", "nowy")
	up.Per100g.Kcal = f(42)
	if err := s.UpdateProduct(ctx, up); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ProductByID(ctx, "hA", "p1")
	if len(got.Aliases) != 1 || got.Aliases[0] != "nowy" || *got.Per100g.Kcal != 42 {
		t.Fatalf("after update %+v", got)
	}
	// alias freed by update can be reused
	if err := s.CreateProduct(ctx, prod("p3", "hA", "ser", "mleczko")); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListProducts(ctx, "hA")
	if err != nil || len(list) != 2 {
		t.Fatalf("list %v %v", list, err)
	}
	if list[0].Name != "mleko" || len(list[0].Aliases) != 1 || list[1].Aliases[0] != "mleczko" {
		t.Fatalf("list %+v", list)
	}
	if _, err := s.ProductByID(ctx, "hB", "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross household: %v", err)
	}
	if err := s.UpdateProduct(ctx, prod("zz", "hA", "x")); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
}

func TestDeleteProductClearsIngredientLinkAndRespectsLog(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	for _, p := range []domain.Product{prod("p1", "hA", "mąka"), prod("p2", "hA", "jajko")} {
		if err := s.CreateProduct(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	r := sampleRecipe("r1", "hA")
	r.Ingredients[0].ProductID = "p1"
	if err := s.CreateRecipe(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProduct(ctx, "hA", "p1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.RecipeByID(ctx, "hA", "r1")
	if err != nil || got.Ingredients[0].ProductID != "" {
		t.Fatalf("%+v %v", got.Ingredients, err)
	}
	if err := s.AddFoodLog(ctx, domain.FoodLogEntry{ID: "l1", HouseholdID: "hA", UserID: "uA", Date: "2026-11-01", ProductID: "p2", Quantity: 2, Unit: "szt", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProduct(ctx, "hA", "p2"); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("want ErrInUse, got %v", err)
	}
	if err := s.DeleteProduct(ctx, "hA", "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSeedProductsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	ps := []domain.Product{prod("p1", "hA", "mleko", "mleczko"), prod("p2", "hA", "ser")}
	ok, err := s.SeedProducts(ctx, "hA", ps)
	if err != nil || !ok {
		t.Fatalf("first: %v %v", ok, err)
	}
	list, _ := s.ListProducts(ctx, "hA")
	if len(list) != 2 {
		t.Fatalf("len %d", len(list))
	}
	for _, p := range list {
		if err := s.DeleteProduct(ctx, "hA", p.ID); err != nil {
			t.Fatal(err)
		}
	}
	ok, err = s.SeedProducts(ctx, "hA", []domain.Product{prod("p9", "hA", "masło")})
	if err != nil || ok {
		t.Fatalf("second: %v %v", ok, err)
	}
	if list, _ := s.ListProducts(ctx, "hA"); len(list) != 0 {
		t.Fatalf("inserted again: %v", list)
	}
	// failed seed rolls back the flag
	bad := []domain.Product{prod("q1", "hB", "a"), prod("q2", "hB", "a")}
	if _, err := s.SeedProducts(ctx, "hB", bad); err == nil {
		t.Fatal("want error")
	}
	if ok, err := s.SeedProducts(ctx, "hB", bad[:1]); err != nil || !ok {
		t.Fatalf("retry after rollback: %v %v", ok, err)
	}
}
