// Package shopping turns a meal plan into a shopping list.
package shopping

import (
	"sort"

	"mealplan/internal/domain"
	"mealplan/internal/units"
)

// Line is one row of a generated list. A nil Quantity means "as needed" (no amount known).
type Line struct {
	Name           string
	NormalizedName string
	Quantity       *float64
	Unit           string
}

type acc struct {
	name, norm, unit string // unit is the base unit for quantified rows
	amount           float64
	hasAmount        bool
}

func key(norm, unit string, hasAmount bool) string {
	if !hasAmount {
		return norm + "|-"
	}
	return norm + "|" + unit
}

// Build scales every recipe entry's ingredients by servings, sums matching rows, subtracts stock and
// returns the rows sorted by name. Free-text entries and unknown recipes contribute nothing.
func Build(entries []domain.PlanEntry, recipes map[string]domain.Recipe, stock []Line) ([]Line, error) {
	rows := map[string]*acc{}
	for _, e := range entries {
		if e.RecipeID == "" {
			continue
		}
		r, ok := recipes[e.RecipeID]
		if !ok {
			continue
		}
		for _, ing := range r.Ingredients {
			if ing.Quantity == nil {
				k := key(ing.NormalizedName, "", false)
				if _, seen := rows[k]; !seen {
					rows[k] = &acc{name: ing.Name, norm: ing.NormalizedName}
				}
				continue
			}
			scaled, err := units.Scale(*ing.Quantity, e.Servings, r.BaseServings)
			if err != nil {
				return nil, err
			}
			amount, unit := units.ToBase(scaled, ing.Unit)
			k := key(ing.NormalizedName, unit, true)
			a, seen := rows[k]
			if !seen {
				a = &acc{name: ing.Name, norm: ing.NormalizedName, unit: unit, hasAmount: true}
				rows[k] = a
			}
			a.amount += amount
		}
	}

	for _, s := range stock {
		if s.Quantity == nil {
			continue
		}
		amount, unit := units.ToBase(*s.Quantity, s.Unit)
		if a, ok := rows[key(s.NormalizedName, unit, true)]; ok {
			a.amount -= amount
			if a.amount <= 0 {
				delete(rows, key(s.NormalizedName, unit, true))
			}
		}
	}

	out := make([]Line, 0, len(rows))
	for _, a := range rows {
		l := Line{Name: a.name, NormalizedName: a.norm}
		if a.hasAmount {
			v, u := units.Humanize(a.amount, a.unit)
			l.Quantity, l.Unit = &v, u
		}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NormalizedName != out[j].NormalizedName {
			return out[i].NormalizedName < out[j].NormalizedName
		}
		return out[i].Unit < out[j].Unit
	})
	return out, nil
}
