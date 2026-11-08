package nutrition

import "mealplan/internal/domain"

type Source string

const (
	SourceRecipe   Source = "recipe"
	SourceComputed Source = "computed"
	SourceMissing  Source = "missing"
)

type Sources struct {
	Kcal, Protein, Carbs, Fat Source
}

type RecipeNutrition struct {
	PerPortion  domain.Macros
	Sources     Sources
	Approximate bool
	Unmatched   []string
}

// ForRecipe computes per-portion values. Values given on the recipe win per
// value; the rest are summed from matched ingredients. Nothing is rounded.
func ForRecipe(r domain.Recipe, m *Matcher) RecipeNutrition {
	var sum [4]float64
	var seen [4]bool
	var out RecipeNutrition
	for _, ing := range r.Ingredients {
		if ing.Quantity == nil {
			continue
		}
		p, ok := m.Match(ing)
		if !ok || !hasMacros(p.Per100g) {
			out.Unmatched = append(out.Unmatched, ing.Name)
			continue
		}
		g, ok := Grams(*ing.Quantity, ing.Unit, p)
		if !ok {
			out.Unmatched = append(out.Unmatched, ing.Name)
			continue
		}
		for i, v := range []*float64{p.Per100g.Kcal, p.Per100g.Protein, p.Per100g.Carbs, p.Per100g.Fat} {
			if v != nil {
				sum[i] += g * *v / 100
				seen[i] = true
			}
		}
	}
	out.Approximate = len(out.Unmatched) > 0
	servings := float64(r.BaseServings)
	if servings < 1 {
		servings = 1
	}
	pick := func(i int, given *float64) (*float64, Source) {
		if given != nil {
			v := *given
			return &v, SourceRecipe
		}
		if seen[i] {
			v := sum[i] / servings
			return &v, SourceComputed
		}
		return nil, SourceMissing
	}
	out.PerPortion.Kcal, out.Sources.Kcal = pick(0, r.Nutrition.Kcal)
	out.PerPortion.Protein, out.Sources.Protein = pick(1, r.Nutrition.Protein)
	out.PerPortion.Carbs, out.Sources.Carbs = pick(2, r.Nutrition.Carbs)
	out.PerPortion.Fat, out.Sources.Fat = pick(3, r.Nutrition.Fat)
	return out
}

func hasMacros(m domain.Macros) bool {
	return m.Kcal != nil || m.Protein != nil || m.Carbs != nil || m.Fat != nil
}
