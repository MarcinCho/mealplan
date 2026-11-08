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
	nutritionPrefix = regexp.MustCompile(`(?i)^\s*(?:wartości\s+odżywcze(?:\s*\(?\s*na\s+porcję\s*\)?)?|wartosci\s+odzywcze(?:\s*\(?\s*na\s+porcje\s*\)?)?|wartości\s+na\s+porcję|wartosci\s+na\s+porcje|na\s+porcję|na\s+porcje|nutrition(?:\s+per\s+serving)?|per\s+serving)\s*(?::|\s[-–—]\s)`)
	numberInText    = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
	kcalRe          = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:kcal|calories|kalorii|kalorie)`)
	gramsAfterRe    = regexp.MustCompile(`^\D{0,20}?(\d+(?:[.,]\d+)?)\s*g\b`)
	gramsBeforeRe   = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*g\b`)
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
	rest := line[loc[1]:]
	var m domain.Macros
	found := false
	// kcal is the number attached to its unit, so a kJ figure in front of it is never used.
	if mm := kcalRe.FindStringSubmatch(rest); mm != nil {
		if n, ok := parseNumber(mm[1]); ok {
			m.Kcal, found = &n, true
		}
	}
	for _, seg := range splitOutsideDecimals(strings.NewReplacer(";", ",", "/", ",").Replace(rest)) {
		low := strings.ToLower(seg)
		// Sub-items ("(w tym nasycone 3 g)", "z czego ...", "of which ...") are cut off, so they never set a main macro
		// and never hide the main value in front of them.
		for _, cut := range []string{"(w tym", "w tym", "z czego", "of which"} {
			if i := strings.Index(low, cut); i >= 0 {
				low = low[:i]
			}
		}
		if strings.TrimSpace(low) == "" || containsAny(low, "nasycon", "saturated") {
			continue
		}
		var dst **float64
		var kw []string
		switch {
		case containsAny(low, "biał", "bial", "protein"):
			dst, kw = &m.Protein, []string{"biał", "bial", "protein"}
		case containsAny(low, "węgl", "wegl", "carb"):
			dst, kw = &m.Carbs, []string{"węgl", "wegl", "carb"}
		case containsAny(low, "tłuszcz", "tluszcz", "fat"):
			dst, kw = &m.Fat, []string{"tłuszcz", "tluszcz", "fat"}
		default:
			continue
		}
		var tok string
		for _, k := range kw {
			if i := strings.Index(low, k); i >= 0 {
				if mm := gramsAfterRe.FindStringSubmatch(low[i+len(k):]); mm != nil {
					tok = mm[1]
				} else if mm := numberInText.FindString(low[i+len(k):]); mm != "" {
					tok = mm
				}
				break
			}
		}
		if tok == "" {
			if mm := gramsBeforeRe.FindStringSubmatch(low); mm != nil {
				tok = mm[1]
			}
		}
		if n, ok := parseNumber(tok); ok && tok != "" {
			*dst, found = &n, true
		}
	}
	return m, found
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
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
