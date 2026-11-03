package importer

import (
	"strings"
	"testing"
)

func names(d Draft) []string {
	var out []string
	for _, i := range d.Ingredients {
		out = append(out, i.Name)
	}
	return out
}

func TestParseTextWithPolishHeadings(t *testing.T) {
	d := ParseText("Placki ziemniaczane\nPorcje: 4\n\nSkładniki:\n- 1 kg ziemniaków\n- 2 jajka\n- sól\n\nPrzygotowanie:\n1. Zetrzyj ziemniaki.\n2. Dodaj jajka i sól.\n3. Smaż na złoto.\n")
	if d.Title != "Placki ziemniaczane" || d.Servings != 4 {
		t.Fatalf("title=%q servings=%d", d.Title, d.Servings)
	}
	if got := strings.Join(names(d), "|"); got != "ziemniaków|jajka|sól" {
		t.Errorf("ingredients=%s", got)
	}
	if len(d.Steps) != 3 || d.Steps[0] != "Zetrzyj ziemniaki." {
		t.Errorf("steps=%q", d.Steps)
	}
}

func TestParseTextEnglishAndMarkdownHeadings(t *testing.T) {
	d := ParseText("# Pancakes\nServes 2\n## Ingredients\n* 200 g flour\n* 2 eggs\n## Method\n1) Mix.\n2) Fry.")
	if d.Title != "Pancakes" || d.Servings != 2 || len(d.Ingredients) != 2 || len(d.Steps) != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestParseTextServingsForms(t *testing.T) {
	for in, want := range map[string]int{
		"Zupa\nPorcje: 6\nSkładniki\n1 l wody":       6,
		"Zupa\n4 porcje\nSkładniki\n1 l wody":        4,
		"Zupa\ndla 3 osób\nSkładniki\n1 l wody":      3,
		"Zupa\nLiczba porcji 8\nSkładniki\n1 l wody": 8,
		"Zupa\nSkładniki\n1 l wody":                  0,
	} {
		if got := ParseText(in).Servings; got != want {
			t.Errorf("%q: servings=%d want %d", in, got, want)
		}
	}
}

func TestParseTextWithoutHeadingsClassifiesLines(t *testing.T) {
	d := ParseText("Naleśniki\n250 g mąki\n2 jajka\n500 ml mleka\nWszystkie składniki dokładnie wymieszaj w misce.\nSmaż na rozgrzanej patelni po minucie z każdej strony.\n")
	if d.Title != "Naleśniki" || len(d.Ingredients) != 3 || len(d.Steps) != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestParseTextMergesWrappedStepsWhenNumbered(t *testing.T) {
	d := ParseText("Zupa\nSkładniki\n1 l wody\nPrzygotowanie\n1. Zagotuj wodę\ni dodaj sól.\n2. Gotuj 10 minut.")
	if len(d.Steps) != 2 || d.Steps[0] != "Zagotuj wodę i dodaj sól." {
		t.Fatalf("steps=%q", d.Steps)
	}
}

func TestParseTextWithoutMarkersEveryLineIsAStep(t *testing.T) {
	d := ParseText("Zupa\nSkładniki\n1 l wody\nPrzygotowanie\nZagotuj wodę.\nDodaj sól.")
	if len(d.Steps) != 2 {
		t.Fatalf("steps=%q", d.Steps)
	}
}

func TestParseTextSkipsSubLabelsAndReportsIgnoredLines(t *testing.T) {
	d := ParseText("Ciasto\nTo jest wstęp do przepisu, który nie należy do żadnej sekcji.\nSkładniki\nNa ciasto:\n200 g mąki\nPrzygotowanie\nWymieszaj.")
	if len(d.Ingredients) != 1 || d.Ingredients[0].Name != "mąki" {
		t.Fatalf("ingredients=%+v", d.Ingredients)
	}
	if len(d.Warnings) == 0 || !strings.Contains(d.Warnings[0], "1") {
		t.Errorf("expected a warning about the skipped intro line: %v", d.Warnings)
	}
}

func TestParseTextKeepsUnparsableIngredientLines(t *testing.T) {
	d := ParseText("Zupa\nSkładniki\n200 g mąki\n???\nPrzygotowanie\nGotuj.")
	if len(d.Ingredients) != 2 || d.Ingredients[1].Raw != "???" {
		t.Fatalf("%+v", d.Ingredients)
	}
}

func TestParseTextEmptyAndFirstLineIngredient(t *testing.T) {
	if d := ParseText("  \n\n"); d.Title != "" || len(d.Ingredients) != 0 || len(d.Steps) != 0 {
		t.Errorf("%+v", d)
	}
	if d := ParseText("200 g mąki\n2 jajka"); d.Title != "" || len(d.Ingredients) != 2 {
		t.Errorf("an ingredient line must not become the title: %+v", d)
	}
}

func TestParseTextNumberedIngredients(t *testing.T) {
	d := ParseText("Naleśniki\nSkładniki:\n1. 200 g mąki\n2) 3 jajka\n2 jajka\n1.5 szklanki mąki\n1,5 kg cukru\n\nPrzygotowanie:\n1. Wymieszaj całość dokładnie.\n")
	if len(d.Ingredients) != 5 {
		t.Fatalf("%+v", d.Ingredients)
	}
	want := []struct {
		raw, name string
		q         float64
	}{
		{"1. 200 g mąki", "mąki", 200}, {"2) 3 jajka", "jajka", 3}, {"2 jajka", "jajka", 2},
		{"1.5 szklanki mąki", "mąki", 1.5}, {"1,5 kg cukru", "cukru", 1.5},
	}
	for i, w := range want {
		g := d.Ingredients[i]
		if g.Raw != w.raw || g.Name != w.name || g.Quantity == nil || *g.Quantity != w.q {
			t.Errorf("%d: %+v", i, g)
		}
	}
}
