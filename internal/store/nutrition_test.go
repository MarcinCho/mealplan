package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func TestNutritionSettingsAndFoodLogIsolation(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	if err := s.CreateProduct(ctx, prod("p1", "hA", "mleko")); err != nil {
		t.Fatal(err)
	}
	def, err := s.NutritionSettings(ctx, "hA", "uA")
	if err != nil || def.Enabled || def.Goal.Kcal != nil {
		t.Fatalf("default %+v %v", def, err)
	}
	if err := s.SaveNutritionSettings(ctx, domain.NutritionSettings{UserID: "uA", HouseholdID: "hA", Enabled: true, Goal: domain.Macros{Kcal: f(2000), Protein: f(100)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveNutritionSettings(ctx, domain.NutritionSettings{UserID: "uA", HouseholdID: "hA", Enabled: true, Goal: domain.Macros{Kcal: f(2200)}}); err != nil {
		t.Fatal(err)
	}
	a, _ := s.NutritionSettings(ctx, "hA", "uA")
	b, _ := s.NutritionSettings(ctx, "hA", "uB")
	if !a.Enabled || *a.Goal.Kcal != 2200 || a.Goal.Protein != nil || b.Enabled || b.Goal.Kcal != nil {
		t.Fatalf("a=%+v b=%+v", a, b)
	}

	e := func(id, user, date string) domain.FoodLogEntry {
		return domain.FoodLogEntry{ID: id, HouseholdID: "hA", UserID: user, Date: date, ProductID: "p1", Quantity: 150, Unit: "g", CreatedAt: now}
	}
	for _, x := range []domain.FoodLogEntry{e("l1", "uA", "2026-11-01"), e("l2", "uA", "2026-11-03"), e("l3", "uA", "2026-11-09"), e("l4", "uB", "2026-11-02")} {
		if err := s.AddFoodLog(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.FoodLog(ctx, "hA", "uA", "2026-11-01", "2026-11-07")
	if err != nil || len(got) != 2 || got[0].ID != "l1" || got[1].ID != "l2" || got[0].Quantity != 150 || got[0].Unit != "g" {
		t.Fatalf("%+v %v", got, err)
	}
	if err := s.DeleteFoodLog(ctx, "hA", "uA", "l4"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.DeleteFoodLog(ctx, "hA", "uA", "l1"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.FoodLog(ctx, "hA", "uA", "2026-11-01", "2026-11-07")
	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
	bad := e("l5", "uA", "2026-11-01")
	bad.Unit = "kg"
	if err := s.AddFoodLog(ctx, bad); err == nil {
		t.Fatal("want error for bad unit")
	}
}
