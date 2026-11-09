package nutrition

import (
	"mealplan/internal/domain"
	"mealplan/internal/units"
)

// measure is one unit expressed as a multiple of the base unit of its dimension.
type measure struct {
	dim    byte // 'm' mass (g), 'v' volume (ml), 'p' pieces (szt)
	factor float64
}

var measures = map[string]measure{
	"g": {'m', 1}, "kg": {'m', 1000},
	"ml": {'v', 1}, "l": {'v', 1000},
	"łyżka": {'v', tablespoonML}, "łyżeczka": {'v', teaspoonML}, "szklanka": {'v', glassML},
	"szt": {'p', 1},
}

// Convert expresses quantity of unit from in unit to for product p, as the pantry needs it: kitchen
// measures (łyżka, łyżeczka, szklanka) count as millilitres, mass and volume convert only with the
// product's density, pieces and mass only with its piece weight. An empty unit means pieces. ok is
// false when the units cannot be compared for this product; unlike Grams nothing is assumed.
func Convert(quantity float64, from, to string, p domain.Product) (float64, bool) {
	from, to = units.CanonUnit(from), units.CanonUnit(to)
	if from == "" {
		from = "szt"
	}
	if to == "" {
		to = "szt"
	}
	if from == to {
		return quantity, true
	}
	f, ok1 := measures[from]
	t, ok2 := measures[to]
	if !ok1 || !ok2 {
		return 0, false
	}
	base := quantity * f.factor
	if f.dim == t.dim {
		return base / t.factor, true
	}
	grams, ok := toGrams(base, f.dim, p)
	if !ok {
		return 0, false
	}
	switch t.dim {
	case 'm':
		return grams / t.factor, true
	case 'v':
		if p.DensityGPerML == nil || *p.DensityGPerML <= 0 {
			return 0, false
		}
		return grams / *p.DensityGPerML / t.factor, true
	case 'p':
		if p.PieceGrams == nil || *p.PieceGrams <= 0 {
			return 0, false
		}
		return grams / *p.PieceGrams / t.factor, true
	}
	return 0, false
}

func toGrams(base float64, dim byte, p domain.Product) (float64, bool) {
	switch dim {
	case 'm':
		return base, true
	case 'v':
		if p.DensityGPerML == nil || *p.DensityGPerML <= 0 {
			return 0, false
		}
		return base * *p.DensityGPerML, true
	case 'p':
		if p.PieceGrams == nil || *p.PieceGrams <= 0 {
			return 0, false
		}
		return base * *p.PieceGrams, true
	}
	return 0, false
}
