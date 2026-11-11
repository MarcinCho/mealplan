package store_test

import (
	"context"
	"errors"
	"fmt"
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

func recipeWith(id, hid string, names ...string) domain.Recipe {
	r := sampleRecipe(id, hid)
	r.Ingredients = nil
	for _, n := range names {
		r.Ingredients = append(r.Ingredients, domain.Ingredient{Name: n, NormalizedName: n, Quantity: f(1), Unit: "szt"})
	}
	return r
}

func mustCreate(t *testing.T, s *store.Store, ps []domain.Product, rs []domain.Recipe) {
	t.Helper()
	ctx := context.Background()
	for _, p := range ps {
		if err := s.CreateProduct(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range rs {
		if err := s.CreateRecipe(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAssignIngredientsAddsAlias(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, []domain.Product{prod("pRed", "hA", "cebula czerwona")},
		[]domain.Recipe{recipeWith("r1", "hA", "czerwona cebula"), recipeWith("r2", "hA", "czerwona cebula")})
	n, owner, err := s.AssignIngredients(ctx, "hA", "pRed", "czerwona cebula", []string{"r1"}, "czerwona cebula")
	if err != nil || n != 1 || owner != "" {
		t.Fatalf("assign: %d %q %v", n, owner, err)
	}
	r1, _ := s.RecipeByID(ctx, "hA", "r1")
	r2, _ := s.RecipeByID(ctx, "hA", "r2")
	if r1.Ingredients[0].ProductID != "pRed" || r2.Ingredients[0].ProductID != "" {
		t.Fatalf("links: %q %q", r1.Ingredients[0].ProductID, r2.Ingredients[0].ProductID)
	}
	p, _ := s.ProductByID(ctx, "hA", "pRed")
	if len(p.Aliases) != 1 || p.Aliases[0] != "czerwona cebula" {
		t.Fatalf("aliases %v", p.Aliases)
	}
	// repeating is a no-op for the alias
	if _, owner, err := s.AssignIngredients(ctx, "hA", "pRed", "czerwona cebula", []string{"r2"}, "czerwona cebula"); err != nil || owner != "" {
		t.Fatalf("repeat: %q %v", owner, err)
	}
	// alias equal to the product name is skipped
	if _, _, err := s.AssignIngredients(ctx, "hA", "pRed", "czerwona cebula", []string{"r2"}, "cebula czerwona"); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.ProductByID(ctx, "hA", "pRed"); len(p.Aliases) != 1 {
		t.Fatalf("aliases %v", p.Aliases)
	}
	if _, _, err := s.AssignIngredients(ctx, "hA", "nope", "x", nil, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown product: %v", err)
	}
}

func TestAssignIngredientsAliasConflict(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, []domain.Product{prod("p1", "hA", "cebula", "cebulka"), prod("p2", "hA", "szalotka")},
		[]domain.Recipe{recipeWith("r1", "hA", "cebulka")})
	n, owner, err := s.AssignIngredients(ctx, "hA", "p2", "cebulka", []string{"r1"}, "cebulka")
	if err != nil || n != 1 || owner != "cebula" {
		t.Fatalf("assign: %d %q %v", n, owner, err)
	}
	r, _ := s.RecipeByID(ctx, "hA", "r1")
	if r.Ingredients[0].ProductID != "p2" {
		t.Fatalf("link %q", r.Ingredients[0].ProductID)
	}
	if p, _ := s.ProductByID(ctx, "hA", "p2"); len(p.Aliases) != 0 {
		t.Fatalf("p2 aliases %v", p.Aliases)
	}
	// alias equal to another product's name is a conflict too
	if _, owner, err := s.AssignIngredients(ctx, "hA", "p2", "cebulka", nil, "cebula"); err != nil || owner != "cebula" {
		t.Fatalf("name conflict: %q %v", owner, err)
	}
}

func TestAssignIngredientsOtherHousehold(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, []domain.Product{prod("p1", "hA", "cebula")},
		[]domain.Recipe{recipeWith("rB", "hB", "cebula")})
	n, _, err := s.AssignIngredients(ctx, "hA", "p1", "cebula", []string{"rB"}, "")
	if err != nil || n != 0 {
		t.Fatalf("assign: %d %v", n, err)
	}
	r, _ := s.RecipeByID(ctx, "hB", "rB")
	if r.Ingredients[0].ProductID != "" {
		t.Fatal("foreign recipe was modified")
	}
}

func TestProductUsage(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, []domain.Product{prod("p1", "hA", "cebula")},
		[]domain.Recipe{recipeWith("r1", "hA", "cebula", "cebulka"), recipeWith("r2", "hA", "cebula")})
	for _, id := range []string{"r1", "r2"} {
		if _, _, err := s.AssignIngredients(ctx, "hA", "p1", "cebula", []string{id}, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.AssignIngredients(ctx, "hA", "p1", "cebulka", []string{"r1"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddToPantry(ctx, "hA", "p1", "g", 100); err != nil {
		t.Fatal(err)
	}
	if err := s.AddFoodLog(ctx, domain.FoodLogEntry{ID: "l1", HouseholdID: "hA", UserID: "uA", Date: "2026-11-01", ProductID: "p1", Quantity: 2, Unit: "szt", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	u, err := s.ProductUsage(ctx, "hA", "p1")
	if err != nil || u != (store.ProductUsage{Recipes: 2, Pantry: 1, FoodLog: 1}) {
		t.Fatalf("usage %+v %v", u, err)
	}
}

func TestIngredientNames(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, nil, []domain.Recipe{recipeWith("r1", "hA", "cebula", "sól"), recipeWith("rB", "hB", "x")})
	got, err := s.IngredientNames(ctx, "hA")
	if err != nil || len(got) != 2 {
		t.Fatalf("names %+v %v", got, err)
	}
	if got[0].RecipeID != "r1" || got[0].RecipeTitle != "Naleśniki" || got[0].NormalizedName != "cebula" || got[0].Quantity == nil || *got[0].Quantity != 1 || got[0].Unit != "szt" {
		t.Fatalf("row %+v", got[0])
	}
}

func pantryOf(t *testing.T, s *store.Store, pid string) []domain.PantryItem {
	t.Helper()
	all, err := s.PantryItems(context.Background(), "hA")
	if err != nil {
		t.Fatal(err)
	}
	var out []domain.PantryItem
	for _, it := range all {
		if it.ProductID == pid {
			out = append(out, it)
		}
	}
	return out
}

func setExpiry(t *testing.T, s *store.Store, it domain.PantryItem, day string) {
	t.Helper()
	it.ExpiresOn = day
	if err := s.SavePantryItem(context.Background(), it); err != nil {
		t.Fatal(err)
	}
}

func TestMergeRepointsEverything(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, []domain.Product{prod("A", "hA", "cebula"), prod("B", "hA", "cebulka", "cebula biała")},
		[]domain.Recipe{recipeWith("r1", "hA", "cebulka")})
	if _, _, err := s.AssignIngredients(ctx, "hA", "B", "cebulka", []string{"r1"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AddFoodLog(ctx, domain.FoodLogEntry{ID: "l1", HouseholdID: "hA", UserID: "uA", Date: "2026-11-01", ProductID: "B", Quantity: 2, Unit: "szt", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertShoppingItem(ctx, domain.ShoppingItem{ID: "s1", HouseholdID: "hA", Week: "2026-W45", Name: "cebulka", NormalizedName: "cebulka", ProductID: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertPlanEntry(ctx, entry("e1", "hA", "2026-11-02", domain.SlotDinner)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddToPantry(ctx, "hA", "B", "g", 100); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyMovements(ctx, "hA", []domain.PantryMovement{{EntryID: "e1", ProductID: "B", Unit: "g", Amount: -10}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.MergeProducts(ctx, "hA", "A", []string{"B"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Repointed != (store.ProductUsage{Recipes: 1, Pantry: 1, FoodLog: 1, Shopping: 1}) || len(res.SkippedAliases) != 0 {
		t.Fatalf("result %+v", res)
	}
	if _, err := s.ProductByID(ctx, "hA", "B"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("B still there: %v", err)
	}
	u, err := s.ProductUsage(ctx, "hA", "A")
	if err != nil || u != (store.ProductUsage{Recipes: 1, Pantry: 1, FoodLog: 1, Shopping: 1}) {
		t.Fatalf("usage %+v %v", u, err)
	}
	if ms, _ := s.MovementsForEntry(ctx, "hA", "e1"); len(ms) != 1 || ms[0].ProductID != "A" {
		t.Fatalf("movements %+v", ms)
	}
	a, _ := s.ProductByID(ctx, "hA", "A")
	if len(a.Aliases) != 2 || a.Aliases[0] != "cebula biała" || a.Aliases[1] != "cebulka" {
		t.Fatalf("aliases %v", a.Aliases)
	}
}

func TestMergeSumsPantrySameUnit(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s) // p1 mąka, p2 jajko
	pa, _ := s.AddToPantry(ctx, "hA", "p1", "g", 200)
	setExpiry(t, s, pa, "2026-12-01")
	pb, _ := s.AddToPantry(ctx, "hA", "p2", "g", 300)
	setExpiry(t, s, pb, "2026-11-20")
	if _, err := s.MergeProducts(ctx, "hA", "p1", []string{"p2"}); err != nil {
		t.Fatal(err)
	}
	got := pantryOf(t, s, "p1")
	if len(got) != 1 || got[0].Quantity == nil || *got[0].Quantity != 500 || got[0].ExpiresOn != "2026-11-20" {
		t.Fatalf("pantry %+v", got)
	}
	if len(pantryOf(t, s, "p2")) != 0 {
		t.Fatal("B stock left")
	}
}

func TestMergeKeepsEmptyExpiryLosing(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_, _ = s.AddToPantry(ctx, "hA", "p1", "g", 1)
	pb, _ := s.AddToPantry(ctx, "hA", "p2", "g", 1)
	setExpiry(t, s, pb, "2026-11-20")
	if _, err := s.MergeProducts(ctx, "hA", "p1", []string{"p2"}); err != nil {
		t.Fatal(err)
	}
	if got := pantryOf(t, s, "p1"); len(got) != 1 || got[0].ExpiresOn != "2026-11-20" {
		t.Fatalf("pantry %+v", got)
	}
}

func TestMergeStapleWins(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_, _ = s.AddToPantry(ctx, "hA", "p1", "szt", 3)
	pb, _ := s.AddToPantry(ctx, "hA", "p2", "szt", 1)
	pb.Staple, pb.Quantity = true, nil
	if err := s.SavePantryItem(ctx, pb); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MergeProducts(ctx, "hA", "p1", []string{"p2"}); err != nil {
		t.Fatal(err)
	}
	if got := pantryOf(t, s, "p1"); len(got) != 1 || !got[0].Staple || got[0].Quantity != nil {
		t.Fatalf("pantry %+v", got)
	}
}

func TestMergeKeepsOtherUnits(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPantry(t, s)
	_, _ = s.AddToPantry(ctx, "hA", "p1", "g", 100)
	_, _ = s.AddToPantry(ctx, "hA", "p2", "szt", 4)
	if _, err := s.MergeProducts(ctx, "hA", "p1", []string{"p2"}); err != nil {
		t.Fatal(err)
	}
	if got := pantryOf(t, s, "p1"); len(got) != 2 {
		t.Fatalf("pantry %+v", got)
	}
}

func TestMergeSkipsAliasOwnedByThird(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	mustCreate(t, s, []domain.Product{prod("A", "hA", "mleko"), prod("B", "hA", "mleczko"), prod("C", "hA", "napój", "mleczko")}, nil)
	res, err := s.MergeProducts(ctx, "hA", "A", []string{"B"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SkippedAliases) != 1 || res.SkippedAliases[0] != "mleczko" {
		t.Fatalf("skipped %v", res.SkippedAliases)
	}
	c, _ := s.ProductByID(ctx, "hA", "C")
	if len(c.Aliases) != 1 || c.Aliases[0] != "mleczko" {
		t.Fatalf("C aliases %v", c.Aliases)
	}
}

func TestMergeCapsAliasesAndRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	var keepAl, absAl []string
	for i := 0; i < store.MaxAliases-1; i++ {
		keepAl = append(keepAl, fmt.Sprintf("k%03d", i))
	}
	absAl = []string{"a1", "a2"}
	mustCreate(t, s, []domain.Product{prod("A", "hA", "keep", keepAl...), prod("B", "hA", "abs", absAl...), prod("X", "hB", "foreign")}, nil)
	if _, err := s.MergeProducts(ctx, "hA", "A", []string{"X"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	if _, err := s.MergeProducts(ctx, "hA", "A", []string{"A"}); err == nil {
		t.Fatal("self merge accepted")
	}
	res, err := s.MergeProducts(ctx, "hA", "A", []string{"B"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.ProductByID(ctx, "hA", "A")
	if len(a.Aliases) != store.MaxAliases || len(res.SkippedAliases) != 2 || res.SkippedAliases[0] != "a2" || res.SkippedAliases[1] != "abs" {
		t.Fatalf("aliases %d skipped %v", len(a.Aliases), res.SkippedAliases)
	}
}
