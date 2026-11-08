package nutrition

import "mealplan/internal/domain"

// DaySummary is one day of an account's intake against its goal.
type DaySummary struct {
	Date         string
	Total        domain.Macros
	Goal         domain.Macros
	Remaining    domain.Macros
	Meals        []MealNutrition
	Log          []LogNutrition
	MissingMeals int
	Approximate  bool
}

// MealNutrition is one planned meal counted as a single portion.
type MealNutrition struct {
	EntryID, Title string
	Slot           domain.Slot
	Values         domain.Macros
	Sources        Sources
	Approximate    bool
	Missing        bool
}

// LogNutrition is one food-log entry with its computed values.
type LogNutrition struct {
	ID, ProductID, Name string
	Quantity            float64
	Unit                string
	Values              domain.Macros
	Missing             bool
}

// BuildDay sums one portion of every planned recipe plus the log entries.
// Entries without a usable recipe and log entries that cannot be converted
// are flagged as missing and add nothing.
func BuildDay(date string, goal domain.Macros, entries []domain.PlanEntry, recipes map[string]domain.Recipe,
	logs []domain.FoodLogEntry, products map[string]domain.Product, m *Matcher) DaySummary {
	d := DaySummary{Date: date, Goal: goal, Meals: []MealNutrition{}, Log: []LogNutrition{}}
	var total [4]float64
	add := func(v domain.Macros) {
		for i, p := range []*float64{v.Kcal, v.Protein, v.Carbs, v.Fat} {
			if p != nil {
				total[i] += *p
			}
		}
	}
	for _, e := range entries {
		meal := MealNutrition{EntryID: e.ID, Title: e.FreeText, Slot: e.Slot}
		r, ok := recipes[e.RecipeID]
		if e.RecipeID != "" && ok {
			meal.Title = r.Title
			n := ForRecipe(r, m)
			meal.Values, meal.Sources, meal.Approximate = n.PerPortion, n.Sources, n.Approximate
			meal.Missing = !hasMacros(n.PerPortion)
		} else {
			meal.Missing = true
			meal.Sources = Sources{SourceMissing, SourceMissing, SourceMissing, SourceMissing}
		}
		if meal.Missing {
			d.MissingMeals++
		}
		if meal.Approximate {
			d.Approximate = true
		}
		add(meal.Values)
		d.Meals = append(d.Meals, meal)
	}
	for _, l := range logs {
		item := LogNutrition{ID: l.ID, ProductID: l.ProductID, Quantity: l.Quantity, Unit: l.Unit}
		p, ok := products[l.ProductID]
		item.Name = p.Name
		g, gok := Grams(l.Quantity, l.Unit, p)
		if !ok || !gok || !hasMacros(p.Per100g) {
			item.Missing = true
		} else {
			scale := func(v *float64) *float64 {
				if v == nil {
					return nil
				}
				x := g * *v / 100
				return &x
			}
			item.Values = domain.Macros{Kcal: scale(p.Per100g.Kcal), Protein: scale(p.Per100g.Protein), Carbs: scale(p.Per100g.Carbs), Fat: scale(p.Per100g.Fat)}
		}
		add(item.Values)
		d.Log = append(d.Log, item)
	}
	ptr := func(i int) *float64 { v := total[i]; return &v }
	d.Total = domain.Macros{Kcal: ptr(0), Protein: ptr(1), Carbs: ptr(2), Fat: ptr(3)}
	rem := func(g *float64, i int) *float64 {
		if g == nil {
			return nil
		}
		v := *g - total[i]
		return &v
	}
	d.Remaining = domain.Macros{Kcal: rem(goal.Kcal, 0), Protein: rem(goal.Protein, 1), Carbs: rem(goal.Carbs, 2), Fat: rem(goal.Fat, 3)}
	return d
}
