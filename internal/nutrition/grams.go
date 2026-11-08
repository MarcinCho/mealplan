// Package nutrition converts ingredient quantities to grams, matches
// ingredients to products and computes per-portion recipe values.
package nutrition

import (
	"mealplan/internal/domain"
	"mealplan/internal/units"
)

const (
	tablespoonML   = 15.0
	teaspoonML     = 5.0
	glassML        = 250.0
	defaultDensity = 1.0
	pinchGrams     = 0.5
	handfulGrams   = 30.0
)

// Grams converts quantity of unit into grams for product p. ok is false when
// the conversion needs data the product lacks or the unit is unknown.
func Grams(quantity float64, unit string, p domain.Product) (float64, bool) {
	density := defaultDensity
	if p.DensityGPerML != nil {
		density = *p.DensityGPerML
	}
	switch units.CanonUnit(unit) {
	case "g":
		return quantity, true
	case "kg":
		return quantity * 1000, true
	case "ml":
		return quantity * density, true
	case "l":
		return quantity * 1000 * density, true
	case "łyżka":
		return quantity * tablespoonML * density, true
	case "łyżeczka":
		return quantity * teaspoonML * density, true
	case "szklanka":
		return quantity * glassML * density, true
	case "szczypta":
		return quantity * pinchGrams, true
	case "garść":
		return quantity * handfulGrams, true
	case "szt", "ząbek", "plaster", "kostka", "opakowanie", "puszka", "kromka", "główka", "pęczek":
		if p.PieceGrams == nil {
			return 0, false
		}
		return quantity * *p.PieceGrams, true
	}
	return 0, false
}
