// Package units normalizes ingredient names and units and scales quantities.
// Only g<->kg and ml<->l are converted; other units are compared as written.
package units

import (
	"errors"
	"math"
	"strings"
)

var ErrBadServings = errors.New("units: servings must be positive")

var aliases = map[string]string{
	"szt": "szt", "szt.": "szt", "sztuka": "szt", "sztuki": "szt", "sztuk": "szt",
	"gram": "g", "gramy": "g", "gramów": "g",
	"kilogram": "kg", "kilogramy": "kg",
	"litr": "l", "litry": "l", "mililitr": "ml", "mililitry": "ml",
}

// NormalizeName lowercases, trims and collapses whitespace so that equal ingredients match.
func NormalizeName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// CanonUnit lowercases a unit and maps common spellings to one form.
func CanonUnit(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	if a, ok := aliases[u]; ok {
		return a
	}
	return u
}

// ToBase converts kg to g and l to ml; other units are returned canonicalized and unchanged.
func ToBase(amount float64, unit string) (float64, string) {
	switch u := CanonUnit(unit); u {
	case "kg":
		return amount * 1000, "g"
	case "l":
		return amount * 1000, "ml"
	default:
		return amount, u
	}
}

// Humanize is the inverse of ToBase for display: 1500 g becomes 1.5 kg.
func Humanize(amount float64, unit string) (float64, string) {
	switch {
	case unit == "g" && amount >= 1000:
		return Round(amount / 1000), "kg"
	case unit == "ml" && amount >= 1000:
		return Round(amount / 1000), "l"
	default:
		return Round(amount), unit
	}
}

// Round keeps three decimals so float noise never reaches the user.
func Round(a float64) float64 { return math.Round(a*1000) / 1000 }

// Scale returns amount * servings / base, rounded.
func Scale(amount float64, servings, base int) (float64, error) {
	if servings <= 0 || base <= 0 {
		return 0, ErrBadServings
	}
	return Round(amount * float64(servings) / float64(base)), nil
}
