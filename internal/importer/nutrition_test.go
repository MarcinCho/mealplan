package importer

import (
	"math"
	"strings"
	"testing"

	"mealplan/internal/domain"
)

func eqf(p *float64, want float64) bool { return p != nil && *p == want }

func TestParseNutritionLine(t *testing.T) {
	m, ok := parseNutritionLine("Wartości na porcję: 520 kcal, białko 32 g, węglowodany 60 g, tłuszcz 14 g")
	if !ok || !eqf(m.Kcal, 520) || !eqf(m.Protein, 32) || !eqf(m.Carbs, 60) || !eqf(m.Fat, 14) {
		t.Fatalf("%+v %v", m, ok)
	}
	m, ok = parseNutritionLine("Wartości na porcję: 410 kcal; tłuszcz 9,5 g")
	if !ok || !eqf(m.Kcal, 410) || !eqf(m.Fat, 9.5) || m.Protein != nil || m.Carbs != nil {
		t.Fatalf("%+v %v", m, ok)
	}
	m, ok = parseNutritionLine("Per serving: 300 kcal, protein 20 g, carbs 30 g, fat 10 g")
	if !ok || !eqf(m.Kcal, 300) || !eqf(m.Protein, 20) || !eqf(m.Carbs, 30) || !eqf(m.Fat, 10) {
		t.Fatalf("%+v %v", m, ok)
	}
	m, ok = parseNutritionLine("Wartości na porcję: 500 kcal, tłuszcz 14 g, w tym kwasy tłuszczowe nasycone 3 g")
	if !ok || !eqf(m.Fat, 14) {
		t.Fatalf("saturated fat overwrote fat: %+v %v", m, ok)
	}
	for _, in := range []string{
		"Wartości na porcję: 500 kcal, tłuszcz 14 g (w tym nasycone 3 g), białko 20 g",
		"Wartości na porcję: 500 kcal, tłuszcz 14 g (w tym kwasy tłuszczowe nasycone 3 g), białko 20 g",
		"Wartości na porcję: 500 kcal, tłuszcz 14 g z czego nasycone 3 g, białko 20 g",
		"Per serving: 500 kcal, fat 14 g of which saturated 3 g, protein 20 g",
	} {
		m, ok = parseNutritionLine(in)
		if !ok || !eqf(m.Fat, 14) || !eqf(m.Protein, 20) {
			t.Errorf("%q: %+v %v", in, m, ok)
		}
	}
	m, ok = parseNutritionLine("Per serving: saturated fat 3 g, fat 14 g")
	if !ok || !eqf(m.Fat, 14) {
		t.Fatalf("saturated first: %+v %v", m, ok)
	}
	if _, ok = parseNutritionLine("Wartości na porcję:"); ok {
		t.Error("no numbers must not be ok")
	}
	if _, ok = parseNutritionLine("2 kg ziemniaków"); ok {
		t.Error("not a nutrition line")
	}
}

func TestParseNutritionLineForms(t *testing.T) {
	for _, c := range []struct {
		in            string
		kcal, protein float64
	}{
		{"Wartości na porcję: 2184 kJ / 520 kcal", 520, 0},
		{"Wartości odżywcze na porcję: 520 kcal, białko 32 g", 520, 32},
		{"Wartości odżywcze (na porcję): 410 kcal", 410, 0},
		{"Per serving - 300 kcal", 300, 0},
		{"Na porcję: 100 kcal", 100, 0},
		{"NUTRITION PER SERVING: 90 kcal, protein 5 g", 90, 5},
	} {
		m, ok := parseNutritionLine(c.in)
		if !ok || !eqf(m.Kcal, c.kcal) || (c.protein != 0 && !eqf(m.Protein, c.protein)) || !isNutritionLine(c.in) {
			t.Errorf("%q: %+v %v", c.in, m, ok)
		}
	}
}

func TestNutritionFromLD(t *testing.T) {
	m := nutritionFromLD(map[string]any{"@type": "NutritionInformation", "calories": "250 kcal", "proteinContent": "10 g",
		"carbohydrateContent": "30,5 g", "fatContent": "8 g"})
	if !eqf(m.Kcal, 250) || !eqf(m.Protein, 10) || !eqf(m.Carbs, 30.5) || !eqf(m.Fat, 8) {
		t.Fatalf("%+v", m)
	}
	if m = nutritionFromLD(map[string]any{"calories": "250 calories"}); !eqf(m.Kcal, 250) {
		t.Errorf("%+v", m)
	}
	if m = nutritionFromLD(map[string]any{"calories": float64(250)}); !eqf(m.Kcal, 250) {
		t.Errorf("%+v", m)
	}
	if m = nutritionFromLD(nil); m != (domain.Macros{}) {
		t.Errorf("%+v", m)
	}
}

func TestParseTextConsumesNutritionLine(t *testing.T) {
	d := ParseText("Zupa\nPorcje: 2\nWartości na porcję: 520 kcal, białko 32 g\n\nSkładniki:\n- 1 l wody\n\nPrzygotowanie:\n1. Gotuj wodę.\n")
	if d.Title != "Zupa" || d.Servings != 2 || !eqf(d.Nutrition.Kcal, 520) || !eqf(d.Nutrition.Protein, 32) {
		t.Fatalf("%+v", d)
	}
	if len(d.Ingredients) != 1 || len(d.Steps) != 1 || len(d.Warnings) != 0 {
		t.Errorf("ing=%v steps=%v warn=%v", d.Ingredients, d.Steps, d.Warnings)
	}
	d = ParseText("Zupa\nWartości na porcję:\nSkładniki:\n- 1 l wody\nPrzygotowanie:\n1. Gotuj.")
	if len(d.Ingredients) != 1 || len(d.Steps) != 1 || d.Nutrition != (domain.Macros{}) {
		t.Errorf("%+v", d)
	}
}

func TestParseHTMLNutritionFromJSONLD(t *testing.T) {
	page := `<html><script type="application/ld+json">{"@type":"Recipe","name":"X","recipeYield":"2","recipeIngredient":["1 l wody"],
"recipeInstructions":"Gotuj.","nutrition":{"@type":"NutritionInformation","calories":"250 kcal","proteinContent":"10 g","carbohydrateContent":"30,5 g","fatContent":"8 g"}}</script></html>`
	d, err := ParseHTML([]byte(page))
	if err != nil || !eqf(d.Nutrition.Kcal, 250) || !eqf(d.Nutrition.Carbs, 30.5) {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestFinishDropsOutOfRangeNutrition(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	d := Draft{Title: "x", Nutrition: domain.Macros{Kcal: v(6000), Protein: v(-1), Carbs: v(math.NaN()), Fat: v(10)}}
	d.finish()
	d.finish()
	if d.Nutrition.Kcal != nil || d.Nutrition.Protein != nil || d.Nutrition.Carbs != nil || !eqf(d.Nutrition.Fat, 10) {
		t.Fatalf("%+v", d.Nutrition)
	}
	n := 0
	for _, w := range d.Warnings {
		if strings.HasPrefix(w, "Pominięto wartość odżywczą") {
			n++
		}
	}
	if n != 3 {
		t.Errorf("warnings=%q", d.Warnings)
	}
}
