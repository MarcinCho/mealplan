package nutrition

import (
	"testing"

	"mealplan/internal/domain"
)

func dayFixture() (domain.Macros, []domain.PlanEntry, map[string]domain.Recipe, []domain.FoodLogEntry, map[string]domain.Product) {
	goal := domain.Macros{Kcal: f(2000), Protein: f(100)}
	entries := []domain.PlanEntry{
		{ID: "e1", Slot: domain.SlotLunch, RecipeID: "r1", Servings: 6, Done: true},
		{ID: "e2", Slot: domain.SlotDinner, RecipeID: "r2", Servings: 1},
		{ID: "e3", Slot: domain.SlotSnack, FreeText: "Owoce", Servings: 1},
	}
	recipes := map[string]domain.Recipe{
		"r1": {ID: "r1", Title: "Zupa", BaseServings: 4, Nutrition: macros(500, 20, 60, 10)},
		"r2": {ID: "r2", Title: "Makaron", BaseServings: 2, Ingredients: []domain.Ingredient{ing("makaron", 200, "g"), ing("woda", 1, "g")}},
	}
	products := map[string]domain.Product{
		"jog":  {ID: "jog", Name: "Jogurt", Per100g: macros(60, 4, 5, 3)},
		"bat":  {ID: "bat", Name: "Baton", Per100g: macros(400, 5, 60, 15), PieceGrams: f(45)},
		"nomc": {ID: "nomc", Name: "Woda"},
		"nopc": {ID: "nopc", Name: "Ser", Per100g: macros(300, 20, 1, 20)},
	}
	logs := []domain.FoodLogEntry{
		{ID: "l1", ProductID: "jog", Quantity: 150, Unit: "g"},
		{ID: "l2", ProductID: "bat", Quantity: 1, Unit: "szt"},
	}
	return goal, entries, recipes, logs, products
}

func TestBuildDaySumsOnePortionPerRecipe(t *testing.T) {
	goal, entries, recipes, logs, products := dayFixture()
	var ps []domain.Product
	for _, p := range products {
		ps = append(ps, p)
	}
	ps = append(ps, domain.Product{ID: "mak", Name: "makaron", Per100g: macros(350, 12, 72, 1.5)})
	d := BuildDay("2026-11-03", goal, entries, recipes, logs, products, NewMatcher(ps))

	if len(d.Meals) != 3 || len(d.Log) != 2 {
		t.Fatalf("meals=%d log=%d", len(d.Meals), len(d.Log))
	}
	if d.Meals[0].Values.Kcal == nil || *d.Meals[0].Values.Kcal != 500 || d.Meals[0].Sources.Kcal != SourceRecipe {
		t.Fatalf("meal0: %+v", d.Meals[0])
	}
	wantMak := 200 * 3.5 / 2
	if d.Meals[1].Values.Kcal == nil || !near(*d.Meals[1].Values.Kcal, wantMak) || !d.Meals[1].Approximate {
		t.Fatalf("meal1: %+v", d.Meals[1])
	}
	if !d.Meals[2].Missing || d.Meals[2].Values.Kcal != nil || d.MissingMeals != 1 || d.Meals[2].Title != "Owoce" {
		t.Fatalf("free text meal: %+v missing=%d", d.Meals[2], d.MissingMeals)
	}
	if !d.Approximate {
		t.Fatal("approximate expected")
	}
	// log: 150 g yogurt = 90 kcal; 1 bar = 45 g = 180 kcal
	if !near(*d.Log[0].Values.Kcal, 90) || !near(*d.Log[1].Values.Kcal, 180) || d.Log[0].Name != "Jogurt" {
		t.Fatalf("log: %+v", d.Log)
	}
	wantKcal := 500 + wantMak + 90 + 180
	if !near(*d.Total.Kcal, wantKcal) {
		t.Fatalf("total kcal %v want %v", *d.Total.Kcal, wantKcal)
	}
	wantProt := 20 + 200*0.12/2 + 6 + 2.25
	if !near(*d.Total.Protein, wantProt) {
		t.Fatalf("total protein %v want %v", *d.Total.Protein, wantProt)
	}
	if !near(*d.Remaining.Kcal, 2000-wantKcal) || !near(*d.Remaining.Protein, 100-wantProt) || d.Remaining.Carbs != nil || d.Remaining.Fat != nil {
		t.Fatalf("remaining: %+v", d.Remaining)
	}
}

func TestBuildDayNegativeRemainingAndMissingLog(t *testing.T) {
	goal, _, _, _, products := dayFixture()
	goal.Kcal = f(100)
	logs := []domain.FoodLogEntry{
		{ID: "a", ProductID: "jog", Quantity: 500, Unit: "g"},
		{ID: "b", ProductID: "nomc", Quantity: 100, Unit: "g"},
		{ID: "c", ProductID: "nopc", Quantity: 1, Unit: "szt"},
		{ID: "d", ProductID: "gone", Quantity: 1, Unit: "g"},
	}
	d := BuildDay("2026-11-03", goal, nil, nil, logs, products, NewMatcher(nil))
	if *d.Remaining.Kcal >= 0 || !near(*d.Remaining.Kcal, 100-300) {
		t.Fatalf("remaining %+v", d.Remaining)
	}
	if d.Log[0].Missing || !d.Log[1].Missing || !d.Log[2].Missing || !d.Log[3].Missing {
		t.Fatalf("missing flags: %+v", d.Log)
	}
	if d.Log[1].Values.Kcal != nil || d.Meals == nil || !d.Approximate {
		t.Fatalf("%+v meals=%v", d.Log[1], d.Meals)
	}
}

func TestBuildDayEmptyAndNilGoal(t *testing.T) {
	d := BuildDay("2026-11-03", domain.Macros{}, nil, nil, nil, nil, NewMatcher(nil))
	if d.Meals == nil || d.Log == nil || d.Remaining.Kcal != nil {
		t.Fatalf("%+v", d)
	}
}

func TestDayFlagsIncompleteValues(t *testing.T) {
	products := map[string]domain.Product{
		"prot": {ID: "prot", Name: "Białko", Per100g: domain.Macros{Protein: f(20)}},
		"none": {ID: "none", Name: "Woda"},
	}
	recipes := map[string]domain.Recipe{"r": {ID: "r", Title: "Sama proteina", BaseServings: 1, Nutrition: domain.Macros{Protein: f(30)}}}
	entries := []domain.PlanEntry{{ID: "e", Slot: domain.SlotLunch, RecipeID: "r"}}
	logs := []domain.FoodLogEntry{{ID: "a", ProductID: "prot", Quantity: 100, Unit: "g"}, {ID: "b", ProductID: "none", Quantity: 100, Unit: "g"}}
	d := BuildDay("2026-11-03", domain.Macros{Kcal: f(2000), Protein: f(100)}, entries, recipes, logs, products, NewMatcher(nil))
	if d.Total.Kcal != nil || d.Remaining.Kcal != nil || !d.Incomplete.Kcal {
		t.Fatalf("kcal must be unknown: %+v %+v %+v", d.Total, d.Remaining, d.Incomplete)
	}
	if d.Total.Protein == nil || !near(*d.Total.Protein, 50) || !near(*d.Remaining.Protein, 50) {
		t.Fatalf("protein: %+v", d.Total)
	}
	if !d.Incomplete.Protein { // the water log item supplies no protein
		t.Fatal("protein incomplete expected")
	}
	if !d.Approximate || d.MissingLog != 1 || d.MissingMeals != 0 || d.Log[0].Missing || !d.Log[1].Missing {
		t.Fatalf("%+v", d)
	}
}

func TestDayCompleteWhenEverythingSupplied(t *testing.T) {
	products := map[string]domain.Product{"jog": {ID: "jog", Name: "Jogurt", Per100g: macros(60, 4, 5, 3)}}
	recipes := map[string]domain.Recipe{"r": {ID: "r", Title: "Zupa", BaseServings: 1, Nutrition: macros(100, 5, 10, 2)}}
	d := BuildDay("2026-11-03", domain.Macros{}, []domain.PlanEntry{{ID: "e", RecipeID: "r"}}, recipes,
		[]domain.FoodLogEntry{{ID: "a", ProductID: "jog", Quantity: 100, Unit: "g"}}, products, NewMatcher(nil))
	if d.Incomplete != (IncompleteFlags{}) || d.Approximate || d.MissingLog != 0 || *d.Total.Kcal != 160 {
		t.Fatalf("%+v", d)
	}
}

func TestEmptyDayIsKnownZero(t *testing.T) {
	d := BuildDay("2026-11-03", domain.Macros{Kcal: f(2000)}, nil, nil, nil, nil, NewMatcher(nil))
	if d.Total.Kcal == nil || *d.Total.Kcal != 0 || d.Remaining.Kcal == nil || *d.Remaining.Kcal != 2000 || d.Remaining.Protein != nil {
		t.Fatalf("%+v %+v", d.Total, d.Remaining)
	}
	if d.Incomplete != (IncompleteFlags{}) || d.Approximate {
		t.Fatalf("%+v", d)
	}
}
