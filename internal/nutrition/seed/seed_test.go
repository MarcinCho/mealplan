package seed

import (
	"math"
	"slices"
	"strings"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/units"
)

func TestSeedProductsAreConsistent(t *testing.T) {
	products, err := Products()
	if err != nil {
		t.Fatal(err)
	}
	if len(products) < 150 {
		t.Fatalf("got %d products, want at least 150", len(products))
	}

	names := map[string]string{}
	for _, p := range products {
		n := units.NormalizeName(p.Name)
		if n == "" || n != p.NormalizedName {
			t.Errorf("%q: NormalizedName %q, want %q", p.Name, p.NormalizedName, n)
		}
		if _, dup := names[n]; dup {
			t.Errorf("duplicate product name %q", n)
		}
		names[n] = p.Name
	}

	aliases := map[string]string{}
	for _, p := range products {
		for _, a := range p.Aliases {
			if a != units.NormalizeName(a) {
				t.Errorf("%s: alias %q is not normalized", p.Name, a)
			}
			if other, ok := names[a]; ok && other != p.Name {
				t.Errorf("%s: alias %q equals the name of product %q", p.Name, a, other)
			}
			if other, ok := aliases[a]; ok {
				t.Errorf("alias %q used by both %q and %q", a, other, p.Name)
			}
			aliases[a] = p.Name
		}
	}

	for _, p := range products {
		m := p.Per100g
		if m.Kcal == nil || m.Protein == nil || m.Carbs == nil || m.Fat == nil {
			t.Errorf("%s: missing macro", p.Name)
			continue
		}
		for label, v := range map[string]float64{"kcal": *m.Kcal, "protein": *m.Protein, "carbs": *m.Carbs, "fat": *m.Fat} {
			if v < 0 {
				t.Errorf("%s: negative %s %v", p.Name, label, v)
			}
		}
		if sum := *m.Protein + *m.Carbs + *m.Fat; sum > 100.0001 {
			t.Errorf("%s: protein+carbs+fat = %v > 100", p.Name, sum)
		}
		atwater := 4**m.Protein + 4**m.Carbs + 9**m.Fat
		tol := math.Max(25, 0.15**m.Kcal)
		if math.Abs(*m.Kcal-atwater) > tol {
			t.Errorf("%s: kcal %v inconsistent with macros (Atwater %v, tolerance %v)", p.Name, *m.Kcal, atwater, tol)
		}
		if p.PieceGrams != nil && !(*p.PieceGrams > 0 && *p.PieceGrams <= 5000) {
			t.Errorf("%s: piece grams %v out of range", p.Name, *p.PieceGrams)
		}
		if p.DensityGPerML != nil && !(*p.DensityGPerML > 0.2 && *p.DensityGPerML <= 2) {
			t.Errorf("%s: density %v out of range", p.Name, *p.DensityGPerML)
		}
	}

	byName := map[string]int{}
	for i, p := range products {
		byName[p.NormalizedName] = i
	}
	required := []string{
		"mąka pszenna", "jajko", "ryż biały", "makaron", "pierś z kurczaka", "mleko", "masło", "oliwa",
		"cukier", "ziemniak", "cebula", "czosnek", "pomidor", "marchew", "ser żółty", "jogurt naturalny",
		"płatki owsiane", "banan", "jabłko", "fasola czerwona", "woda",
	}
	for _, r := range required {
		if _, ok := byName[r]; !ok {
			t.Errorf("required product %q missing", r)
		}
	}
	if i, ok := byName["jajko"]; ok && products[i].PieceGrams == nil {
		t.Error("jajko needs piece grams")
	}
	if i, ok := byName["czosnek"]; ok && products[i].PieceGrams == nil {
		t.Error("czosnek needs piece grams (clove)")
	}
	if i, ok := byName["oliwa"]; ok && products[i].DensityGPerML == nil {
		t.Error("oliwa needs density")
	}
}

func TestSeedRedOnionIsSeparateFromOnion(t *testing.T) {
	products, err := Products()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]domain.Product{}
	for _, p := range products {
		byName[p.NormalizedName] = p
	}
	onion, ok := byName["cebula"]
	if !ok {
		t.Fatal("cebula missing")
	}
	for _, a := range onion.Aliases {
		if strings.Contains(a, "czerwon") {
			t.Errorf("cebula has red alias %q", a)
		}
	}
	red, ok := byName["cebula czerwona"]
	if !ok {
		t.Fatal("cebula czerwona missing")
	}
	if !slices.Contains(red.Aliases, "czerwona cebula") {
		t.Errorf("cebula czerwona aliases = %v, want czerwona cebula", red.Aliases)
	}
}
