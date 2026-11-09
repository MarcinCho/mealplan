package shopping_test

import (
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/shopping"
)

func q(v float64) *float64 { return &v }

func ing(name string, qty *float64, unit string) domain.Ingredient {
	return domain.Ingredient{Name: name, NormalizedName: name, Quantity: qty, Unit: unit}
}

func recipes(rs ...domain.Recipe) map[string]domain.Recipe {
	m := map[string]domain.Recipe{}
	for _, r := range rs {
		m[r.ID] = r
	}
	return m
}

func entry(recipeID string, servings int) domain.PlanEntry {
	return domain.PlanEntry{ID: "e", RecipeID: recipeID, Servings: servings}
}

func none(domain.Ingredient) string { return "" }

func byNorm(ing domain.Ingredient) string { return "p-" + ing.NormalizedName }

func find(lines []shopping.Line, name, unit string) *shopping.Line {
	for i := range lines {
		if lines[i].NormalizedName == name && lines[i].Unit == unit {
			return &lines[i]
		}
	}
	return nil
}

func TestScalesBySevings(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 4, Ingredients: []domain.Ingredient{ing("mąka", q(200), "g")}}
	got, err := shopping.Build([]domain.PlanEntry{entry("r", 3)}, recipes(r), none, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := find(got, "mąka", "g")
	if l == nil || l.Quantity == nil || *l.Quantity != 150 {
		t.Fatalf("scaled: %+v", got)
	}
}

func TestSumsAcrossRecipesAndMixedMetricUnits(t *testing.T) {
	a := domain.Recipe{ID: "a", BaseServings: 2, Ingredients: []domain.Ingredient{ing("mąka", q(0.5), "kg"), ing("mleko", q(0.5), "l")}}
	b := domain.Recipe{ID: "b", BaseServings: 2, Ingredients: []domain.Ingredient{ing("mąka", q(300), "g"), ing("mleko", q(700), "ml")}}
	got, err := shopping.Build([]domain.PlanEntry{entry("a", 2), entry("b", 2)}, recipes(a, b), none, nil)
	if err != nil {
		t.Fatal(err)
	}
	if l := find(got, "mąka", "g"); l == nil || *l.Quantity != 800 {
		t.Fatalf("flour: %+v", got)
	}
	if l := find(got, "mleko", "l"); l == nil || *l.Quantity != 1.2 {
		t.Fatalf("milk must humanize to 1.2 l: %+v", got)
	}
}

func TestHumanizesLargeTotals(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("ziemniaki", q(1500), "g")}}
	got, _ := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), none, nil)
	if l := find(got, "ziemniaki", "kg"); l == nil || *l.Quantity != 1.5 {
		t.Fatalf("1500 g must show as 1.5 kg: %+v", got)
	}
}

func TestIncompatibleUnitsStaySeparate(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("cebula", q(2), "szt"), ing("cebula", q(200), "g")}}
	got, _ := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), none, nil)
	if find(got, "cebula", "szt") == nil || find(got, "cebula", "g") == nil || len(got) != 2 {
		t.Fatalf("separate rows expected: %+v", got)
	}
}

func TestIngredientWithoutQuantityAppearsOnce(t *testing.T) {
	a := domain.Recipe{ID: "a", BaseServings: 2, Ingredients: []domain.Ingredient{ing("sól", nil, "")}}
	b := domain.Recipe{ID: "b", BaseServings: 2, Ingredients: []domain.Ingredient{ing("sól", nil, "szczypta")}}
	got, err := shopping.Build([]domain.PlanEntry{entry("a", 4), entry("b", 1), entry("a", 2)}, recipes(a, b), none, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range got {
		if l.NormalizedName == "sól" {
			n++
			if l.Quantity != nil {
				t.Fatalf("no quantity expected: %+v", l)
			}
		}
	}
	if n != 1 {
		t.Fatalf("salt must be listed once, got %d: %+v", n, got)
	}
}

func TestNoQuantityDoesNotSwallowQuantifiedRow(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("masło", q(50), "g"), ing("masło", nil, "")}}
	got, _ := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), none, nil)
	if find(got, "masło", "g") == nil || len(got) != 2 {
		t.Fatalf("both rows expected: %+v", got)
	}
}

func TestFreeTextAndUnknownRecipesAreSkipped(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("jajka", q(2), "szt")}}
	free := domain.PlanEntry{ID: "f", FreeText: "Pizza", Servings: 2}
	got, err := shopping.Build([]domain.PlanEntry{free, entry("missing", 1), entry("r", 1)}, recipes(r), none, nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("skip: %v %+v", err, got)
	}
}

func TestBadServingsAreAnError(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 0, Ingredients: []domain.Ingredient{ing("mąka", q(1), "g")}}
	if _, err := shopping.Build([]domain.PlanEntry{entry("r", 2)}, recipes(r), none, nil); err == nil {
		t.Fatal("zero base servings must be an error, not a division by zero")
	}
	ok := domain.Recipe{ID: "ok", BaseServings: 2, Ingredients: []domain.Ingredient{ing("mąka", q(1), "g")}}
	if _, err := shopping.Build([]domain.PlanEntry{entry("ok", 0)}, recipes(ok), none, nil); err == nil {
		t.Fatal("zero entry servings must be an error")
	}
}

func TestOutputIsSorted(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("żółtko", q(1), "szt"), ing("jabłko", q(1), "szt"), ing("banan", q(1), "szt")}}
	got, _ := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), none, nil)
	if len(got) != 3 || got[0].NormalizedName != "banan" || got[1].NormalizedName != "jabłko" || got[2].NormalizedName != "żółtko" {
		t.Fatalf("order: %+v", got)
	}
}

func TestBuildSkipsDoneEntries(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("mąka", q(500), "g")}}
	done := entry("r", 1)
	done.Done = true
	got, err := shopping.Build([]domain.PlanEntry{done}, recipes(r), none, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("done entry must add nothing: %v %+v", err, got)
	}
	got, _ = shopping.Build([]domain.PlanEntry{done, entry("r", 1)}, recipes(r), none, nil)
	if l := find(got, "mąka", "g"); len(got) != 1 || l == nil || *l.Quantity != 500 {
		t.Fatalf("only the open entry counts: %+v", got)
	}
}

func TestBuildSubtractsStockByProductAndUnit(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("mąka", q(500), "g")}}
	run := func(stock ...shopping.StockLine) shopping.Line {
		got, err := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), byNorm, stock)
		if err != nil || len(got) != 1 {
			t.Fatalf("build: %v %+v", err, got)
		}
		return got[0]
	}
	l := run(shopping.StockLine{ProductID: "p-mąka", Quantity: q(200), Unit: "g"})
	if l.Unit != "g" || *l.Quantity != 300 || l.Have == nil || *l.Have != 200 || l.ProductID != "p-mąka" {
		t.Fatalf("partial: %+v", l)
	}
	l = run(shopping.StockLine{ProductID: "p-mąka", Quantity: q(2000), Unit: "g"})
	if l.Quantity == nil || *l.Quantity != 0 || l.Have == nil || *l.Have != 500 || l.Unit != "g" {
		t.Fatalf("covered: %+v", l)
	}
	l = run(shopping.StockLine{ProductID: "p-mąka", Quantity: q(2), Unit: "szt"})
	if *l.Quantity != 500 || l.Have != nil {
		t.Fatalf("other unit must not subtract: %+v", l)
	}
	l = run(shopping.StockLine{ProductID: "p-inne", Quantity: q(900), Unit: "g"})
	if *l.Quantity != 500 || l.Have != nil {
		t.Fatalf("other product must not subtract: %+v", l)
	}
}

func TestBuildDropsStaples(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("sól", nil, ""), ing("mąka", q(100), "g")}}
	got, _ := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), byNorm, []shopping.StockLine{{ProductID: "p-sól", Unit: "szt", Staple: true}})
	if len(got) != 1 || got[0].NormalizedName != "mąka" {
		t.Fatalf("staple must not appear: %+v", got)
	}
}

func TestBuildMatchesUnitlessIngredientAsPieces(t *testing.T) {
	r := domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("jajka", q(2), "")}}
	got, _ := shopping.Build([]domain.PlanEntry{entry("r", 1)}, recipes(r), byNorm, []shopping.StockLine{{ProductID: "p-jajka", Quantity: q(2), Unit: "szt"}})
	if len(got) != 1 || got[0].Quantity == nil || *got[0].Quantity != 0 || got[0].Have == nil || *got[0].Have != 2 {
		t.Fatalf("szt stock must offset a unit-less amount: %+v", got)
	}
}

func TestBuildSkipsPrepAndOutEntries(t *testing.T) {
	r := recipes(domain.Recipe{ID: "r", BaseServings: 1, Ingredients: []domain.Ingredient{ing("ryż", q(100), "g")}})
	entries := []domain.PlanEntry{
		{ID: "a", RecipeID: "r", Servings: 1, Kind: domain.EntryPrep},
		{ID: "b", Kind: domain.EntryOut, Servings: 1},
		{ID: "c", RecipeID: "r", Servings: 1, Kind: domain.EntryMeal},
		{ID: "d", RecipeID: "r", Servings: 1},
	}
	lines, err := shopping.Build(entries, r, none, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := find(lines, "ryż", "g")
	if len(lines) != 1 || l == nil || *l.Quantity != 200 {
		t.Fatalf("%+v", lines)
	}
}
