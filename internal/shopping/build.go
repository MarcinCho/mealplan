// Package shopping turns a meal plan into a shopping list.
package shopping

import (
	"math"
	"sort"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/units"
)

// Line is one row of a generated list. A nil Quantity means "as needed" (no amount known).
type Line struct {
	Name           string
	NormalizedName string
	Quantity       *float64
	Unit           string
	ProductID      string   // matched product, "" when unknown
	Have           *float64 // amount already in stock that was subtracted, in Unit; nil when none
}

// StockLine is pantry stock as Build needs it; Quantity is nil for a staple. Product, when known,
// lets an amount in another unit be compared with this stock (nutrition.Convert).
type StockLine struct {
	ProductID string
	Quantity  *float64
	Unit      string
	Staple    bool
	Product   domain.Product
}

type acc struct {
	name, norm, unit string // unit is the base unit for quantified rows
	productID        string
	have             float64 // unit is the base unit for quantified rows
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
// returns the rows sorted by name. Prep and eaten-out entries, free-text entries, done entries and unknown recipes contribute
// nothing; ingredients of a staple product are left out. A row fully covered by stock stays with
// Quantity 0 and Have set.
func Build(entries []domain.PlanEntry, recipes map[string]domain.Recipe, productOf func(domain.Ingredient) string, stock []StockLine) ([]Line, error) {
	staples := map[string]bool{}
	have := map[string]float64{}
	stockUnits := map[string][]string{} // product -> its stock units, in stock order
	products := map[string]domain.Product{}
	for _, s := range stock {
		if s.ProductID == "" {
			continue
		}
		if s.Staple {
			staples[s.ProductID] = true
			continue
		}
		if s.Quantity != nil {
			amount, unit := units.ToBase(*s.Quantity, s.Unit)
			k := s.ProductID + "|" + unit
			if _, seen := have[k]; !seen {
				stockUnits[s.ProductID] = append(stockUnits[s.ProductID], unit)
			}
			have[k] += amount
			if s.Product.ID != "" {
				products[s.ProductID] = s.Product
			}
		}
	}

	rows := map[string]*acc{}
	for _, e := range entries {
		if (e.Kind != "" && e.Kind != domain.EntryMeal) || e.RecipeID == "" || e.Done {
			continue
		}
		r, ok := recipes[e.RecipeID]
		if !ok {
			continue
		}
		for _, ing := range r.Ingredients {
			pid := productOf(ing)
			if pid != "" && staples[pid] {
				continue
			}
			if ing.Quantity == nil {
				k := key(ing.NormalizedName, "", false)
				if _, seen := rows[k]; !seen {
					rows[k] = &acc{name: ing.Name, norm: ing.NormalizedName, productID: pid}
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
				a = &acc{name: ing.Name, norm: ing.NormalizedName, unit: unit, hasAmount: true, productID: pid}
				rows[k] = a
			}
			a.amount += amount
		}
	}

	// Rows of one product share its stock; walk them in a stable order.
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := rows[k]
		if !a.hasAmount || a.productID == "" {
			continue
		}
		stockUnit := a.unit
		if stockUnit == "" {
			stockUnit = "szt" // a unit-less amount counts pieces
		}
		// Same unit first, then any other stock unit of the product that the amount converts to.
		for _, su := range append([]string{stockUnit}, stockUnits[a.productID]...) {
			if a.amount <= 0 {
				break
			}
			sk := a.productID + "|" + su
			if have[sk] <= 0 {
				continue
			}
			need, ok := a.amount, true
			if su != stockUnit {
				need, ok = nutrition.Convert(a.amount, stockUnit, su, products[a.productID])
			}
			if !ok || need <= 0 {
				continue
			}
			take := math.Min(have[sk], need)
			have[sk] -= take
			taken := take * a.amount / need // back in the row's unit
			a.amount -= taken
			a.have += taken
		}
	}

	out := make([]Line, 0, len(rows))
	for _, a := range rows {
		l := Line{Name: a.name, NormalizedName: a.norm, ProductID: a.productID}
		if a.hasAmount {
			if units.Round(a.amount) > 0 {
				v, u := units.Humanize(a.amount, a.unit)
				l.Quantity, l.Unit = &v, u
				if a.have > 0 {
					h := units.Round(a.have * v / a.amount)
					l.Have = &h
				}
			} else {
				// Fully covered: keep the row, nothing left to buy.
				h, u := units.Humanize(a.have, a.unit)
				zero := 0.0
				l.Quantity, l.Unit, l.Have = &zero, u, &h
			}
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
