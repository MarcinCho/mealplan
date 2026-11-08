package importer

import (
	"math"
	"regexp"
	"strings"

	"mealplan/internal/domain"
)

// Limits per portion; they match the recipe validation so a reviewed draft can always be saved.
const (
	maxKcal  = 5000
	maxGrams = 500
)

var (
	nutritionPrefix = regexp.MustCompile(`(?i)^\s*(?:wartości\s+odżywcze|wartosci\s+odzywcze|wartości\s+na\s+porcję|wartosci\s+na\s+porcje|nutrition(?:\s+per\s+serving)?|per\s+serving)\s*:`)
	numberInText    = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

// firstNumber returns the first decimal number in s ("30,5 g" -> 30.5).
func firstNumber(s string) (float64, bool) {
	tok := numberInText.FindString(s)
	if tok == "" {
		return 0, false
	}
	return parseNumber(tok)
}

// isNutritionLine reports whether the line is introduced as per-portion values, with or without numbers.
func isNutritionLine(line string) bool { return nutritionPrefix.MatchString(line) }

// parseNutritionLine reads "Wartości na porcję: 520 kcal, białko 32 g, ..." (any order, any subset).
func parseNutritionLine(line string) (domain.Macros, bool) {
	loc := nutritionPrefix.FindStringIndex(line)
	if loc == nil {
		return domain.Macros{}, false
	}
	var m domain.Macros
	found := false
	for _, part := range strings.Split(line[loc[1]:], ";") {
		// A comma separates values unless it is a decimal comma between digits.
		for _, seg := range splitOutsideDecimals(part) {
			low := strings.ToLower(seg)
			n, ok := firstNumber(low)
			if !ok {
				continue
			}
			var dst **float64
			switch {
			case strings.Contains(low, "kcal") || strings.Contains(low, "kalori") || strings.Contains(low, "calor"):
				dst = &m.Kcal
			case strings.Contains(low, "biał") || strings.Contains(low, "bial") || strings.Contains(low, "protein"):
				dst = &m.Protein
			case strings.Contains(low, "węgl") || strings.Contains(low, "wegl") || strings.Contains(low, "carb"):
				dst = &m.Carbs
			case strings.Contains(low, "tłuszcz") || strings.Contains(low, "tluszcz") || strings.Contains(low, "fat"):
				dst = &m.Fat
			default:
				continue
			}
			v := n
			*dst, found = &v, true
		}
	}
	return m, found
}

// splitOutsideDecimals splits on commas that are not between two digits.
func splitOutsideDecimals(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' && !(i > 0 && i+1 < len(s) && isDigit(s[i-1]) && isDigit(s[i+1])) {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func ldNumber(v any) *float64 {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case string:
		if strings.Contains(strings.ToLower(x), "kj") {
			return nil
		}
		f, ok := firstNumber(x)
		if !ok {
			return nil
		}
		n = f
	default:
		return nil
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return &n
}

// nutritionFromLD reads a schema.org NutritionInformation object (per serving).
func nutritionFromLD(v any) domain.Macros {
	obj, ok := v.(map[string]any)
	if !ok {
		return domain.Macros{}
	}
	return domain.Macros{Kcal: ldNumber(obj["calories"]), Protein: ldNumber(obj["proteinContent"]),
		Carbs: ldNumber(obj["carbohydrateContent"]), Fat: ldNumber(obj["fatContent"])}
}

// limitNutrition nils every value outside the recipe limits and warns once per value.
func (d *Draft) limitNutrition() {
	for _, c := range []struct {
		label string
		v     **float64
		limit float64
	}{{"kcal", &d.Nutrition.Kcal, maxKcal}, {"białko", &d.Nutrition.Protein, maxGrams},
		{"węglowodany", &d.Nutrition.Carbs, maxGrams}, {"tłuszcz", &d.Nutrition.Fat, maxGrams}} {
		if p := *c.v; p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 0 || *p > c.limit) {
			*c.v = nil
			d.warn("Pominięto wartość odżywczą spoza zakresu: %s.", c.label)
		}
	}
}
