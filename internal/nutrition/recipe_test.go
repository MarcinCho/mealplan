package nutrition

import (
	"testing"

	"mealplan/internal/domain"
)

func macros(k, p, c, fa float64) domain.Macros {
	return domain.Macros{Kcal: f(k), Protein: f(p), Carbs: f(c), Fat: f(fa)}
}

func testMatcher() *Matcher {
	return NewMatcher([]domain.Product{
		{ID: "mak", Name: "makaron", Per100g: macros(350, 12, 72, 1.5)},
		{ID: "oil", Name: "oliwa", Per100g: macros(884, 0, 0, 100), DensityGPerML: f(0.92)},
		{ID: "nomacro", Name: "woda"},
	})
}

func ing(name string, q float64, unit string) domain.Ingredient {
	return domain.Ingredient{Name: name, Quantity: f(q), Unit: unit}
}

func TestForRecipeComputesPerPortion(t *testing.T) {
	r := domain.Recipe{BaseServings: 4, Ingredients: []domain.Ingredient{ing("makaron", 400, "g"), ing("oliwa", 2, "łyżka")}}
	n := ForRecipe(r, testMatcher())
	want := (400*3.5 + 27.6*8.84) / 4
	if n.PerPortion.Kcal == nil || !near(*n.PerPortion.Kcal, want) {
		t.Fatalf("kcal %v want %v", n.PerPortion.Kcal, want)
	}
	s := n.Sources
	if s.Kcal != SourceComputed || s.Protein != SourceComputed || s.Carbs != SourceComputed || s.Fat != SourceComputed || n.Approximate {
		t.Fatalf("%+v approx=%v", s, n.Approximate)
	}
}

func TestGivenValuesWinPerValue(t *testing.T) {
	r := domain.Recipe{BaseServings: 4, Nutrition: domain.Macros{Kcal: f(520), Protein: f(32)},
		Ingredients: []domain.Ingredient{ing("makaron", 400, "g")}}
	n := ForRecipe(r, testMatcher())
	if *n.PerPortion.Kcal != 520 || *n.PerPortion.Protein != 32 || n.Sources.Kcal != SourceRecipe || n.Sources.Protein != SourceRecipe {
		t.Fatalf("%+v", n)
	}
	if n.Sources.Carbs != SourceComputed || n.Sources.Fat != SourceComputed || !near(*n.PerPortion.Carbs, 72) {
		t.Fatalf("%+v", n)
	}
}

func TestUnmatchedMakesApproximate(t *testing.T) {
	r := domain.Recipe{BaseServings: 2, Ingredients: []domain.Ingredient{ing("makaron", 200, "g"), ing("jajka", 3, "szt")}}
	n := ForRecipe(r, testMatcher())
	if !n.Approximate || len(n.Unmatched) != 1 || n.Unmatched[0] != "jajka" {
		t.Fatalf("%+v", n)
	}
	if n.PerPortion.Kcal == nil || !near(*n.PerPortion.Kcal, 350) {
		t.Fatalf("kcal %v", n.PerPortion.Kcal)
	}
}

func TestNoQuantityIsZeroWithoutFlag(t *testing.T) {
	r := domain.Recipe{BaseServings: 2, Ingredients: []domain.Ingredient{{Name: "sól"}}}
	n := ForRecipe(r, testMatcher())
	if n.Approximate || len(n.Unmatched) != 0 {
		t.Fatalf("%+v", n)
	}
}

func TestNothingComputable(t *testing.T) {
	r := domain.Recipe{BaseServings: 2, Ingredients: []domain.Ingredient{ing("jajka", 3, "szt")}}
	n := ForRecipe(r, testMatcher())
	m := SourceMissing
	if n.Sources != (Sources{m, m, m, m}) || n.PerPortion != (domain.Macros{}) {
		t.Fatalf("%+v", n)
	}
}

func TestProductWithoutMacrosIsUnmatched(t *testing.T) {
	r := domain.Recipe{BaseServings: 1, Ingredients: []domain.Ingredient{ing("woda", 1, "l")}}
	n := ForRecipe(r, testMatcher())
	if len(n.Unmatched) != 1 || n.Unmatched[0] != "woda" {
		t.Fatalf("%+v", n)
	}
}
