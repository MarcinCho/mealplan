package importer

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The race detector slows this code about ten times, so the time bound is wider there. The bound only has to
// separate "linear" (about 0.1 s) from the old quadratic merge (about 8 s); the structural checks do the rest.
func timeBound() time.Duration {
	if raceEnabled {
		return 15 * time.Second
	}
	return 3 * time.Second
}

func hasWarning(d Draft, sub string) bool {
	return strings.Contains(strings.Join(d.Warnings, "|"), sub)
}

func TestParseHTMLBoundsWorkOnHugePage(t *testing.T) {
	page := "<html><body><p>Składniki</p><p>200 g mąki</p><p>2 jajka</p><p>Przygotowanie</p><p>1. Start</p>" +
		strings.Repeat("<p>x</p>", 2<<20/8) + "</body></html>"
	start := time.Now()
	d, err := ParseHTML([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > timeBound() {
		t.Errorf("took %v", el)
	}
	if len(d.Steps) > maxSteps+10 || len(d.Ingredients) != 2 || !hasWarning(d, "pierwsze") {
		t.Errorf("steps=%d ingredients=%d warnings=%v", len(d.Steps), len(d.Ingredients), d.Warnings)
	}
}

func TestParseTextBoundsWorkOnLargePaste(t *testing.T) {
	text := "Składniki\n200 g mąki\n2 jajka\nPrzygotowanie\n1. Start\n" + strings.Repeat("x\n", MaxTextBytes/2-20)
	start := time.Now()
	d := ParseText(text)
	if el := time.Since(start); el > timeBound() {
		t.Errorf("took %v", el)
	}
	if len(d.Steps) > maxSteps+10 || !hasWarning(d, "pierwsze") {
		t.Errorf("steps=%d warnings=%v", len(d.Steps), d.Warnings)
	}
}

func TestParseTextCapsIngredientsAndSteps(t *testing.T) {
	var b strings.Builder
	b.WriteString("Składniki\n")
	for i := 0; i < 1000; i++ {
		b.WriteString("200 g mąki\n")
	}
	b.WriteString("Przygotowanie\n")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "%d. Zrób coś ważnego\n", i%99+1)
	}
	d := ParseText(b.String())
	if len(d.Ingredients) > maxIngredients+10 || len(d.Steps) > maxSteps+10 {
		t.Errorf("ingredients=%d steps=%d", len(d.Ingredients), len(d.Steps))
	}
	if !hasWarning(d, "składniki ponad limit") || !hasWarning(d, "kroki ponad limit") {
		t.Errorf("warnings=%v", d.Warnings)
	}
}

func TestJSONLDListsAreCapped(t *testing.T) {
	var ing, st []string
	for i := 0; i < 3000; i++ {
		ing = append(ing, `"200 g mąki"`)
		st = append(st, `"Krok"`)
	}
	page := `<script type="application/ld+json">{"@type":"Recipe","name":"R","recipeIngredient":[` + strings.Join(ing, ",") +
		`],"recipeInstructions":[` + strings.Join(st, ",") + `]}</script>`
	d, err := ParseHTML([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Ingredients) > maxIngredients+10 || len(d.Steps) > maxSteps+10 || !hasWarning(d, "składniki ponad limit") {
		t.Errorf("ingredients=%d steps=%d %v", len(d.Ingredients), len(d.Steps), d.Warnings)
	}
}
