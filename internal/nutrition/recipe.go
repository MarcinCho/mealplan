package nutrition

import (
	"slices"
	"strings"

	"mealplan/internal/domain"
)

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
	type lack struct {
		name string
		miss [4]bool
	}
	var lacks []lack
	var out RecipeNutrition
	for _, ing := range r.Ingredients {
		p, ok := m.Match(ing)
		if ing.Quantity == nil {
			if !negligibleWithoutQuantity(ing, p, ok) {
				out.Unmatched = append(out.Unmatched, ing.Name)
			}
			continue
		}
		if !ok || !hasMacros(p.Per100g) {
			out.Unmatched = append(out.Unmatched, ing.Name)
			continue
		}
		g, ok := Grams(*ing.Quantity, ing.Unit, p)
		if !ok {
			out.Unmatched = append(out.Unmatched, ing.Name)
			continue
		}
		l := lack{name: ing.Name}
		for i, v := range []*float64{p.Per100g.Kcal, p.Per100g.Protein, p.Per100g.Carbs, p.Per100g.Fat} {
			if v != nil {
				sum[i] += g * *v / 100
				seen[i] = true
			} else {
				l.miss[i] = true
			}
		}
		lacks = append(lacks, l)
	}
	// An ingredient lacking a macro that is reported as computed understates it.
	given := [4]bool{r.Nutrition.Kcal != nil, r.Nutrition.Protein != nil, r.Nutrition.Carbs != nil, r.Nutrition.Fat != nil}
	for _, l := range lacks {
		for i := range l.miss {
			if l.miss[i] && seen[i] && !given[i] && !slices.Contains(out.Unmatched, l.name) {
				out.Unmatched = append(out.Unmatched, l.name)
			}
		}
	}
	// Approximate only when a computed value could be understated.
	out.Approximate = len(out.Unmatched) > 0 && !(given[0] && given[1] && given[2] && given[3])
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

var negligibleMarkers = []string{"do smaku", "do dekoracji", "do posypania", "opcjonalnie"}

// negligibleWithoutQuantity reports whether an ingredient without a quantity
// may count as 0: it matched a zero-kcal product (salt, water) or is marked
// as to-taste/decoration/optional in its name or note.
func negligibleWithoutQuantity(ing domain.Ingredient, p domain.Product, matched bool) bool {
	if matched && p.Per100g.Kcal != nil && *p.Per100g.Kcal == 0 {
		return true
	}
	text := strings.ToLower(ing.Name + " " + ing.Note)
	for _, mk := range negligibleMarkers {
		if strings.Contains(text, mk) {
			return true
		}
	}
	return false
}
