package importer

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseHTMLJSONLDGraph(t *testing.T) {
	d, err := ParseHTML(fixture(t, "jsonld_graph.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Zupa pomidorowa & ryż" || d.Servings != 4 {
		t.Errorf("title=%q servings=%d", d.Title, d.Servings)
	}
	if got := strings.Join(names(d), "|"); got != "bulionu|koncentratu pomidorowego|ryżu|sól" {
		t.Errorf("ingredients=%s", got)
	}
	want := []string{"Zagotuj bulion.", "Dodaj koncentrat i ryż.", "Gotuj 15 minut."}
	if strings.Join(d.Steps, "|") != strings.Join(want, "|") {
		t.Errorf("steps=%q", d.Steps)
	}
	d.finish()
	if got := strings.Join(d.Tags, ","); got != "zupa,obiad,szybkie,dania główne" {
		t.Errorf("tags=%q", got)
	}
}

func TestParseHTMLJSONLDPlainWithStringInstructions(t *testing.T) {
	d, err := ParseHTML(fixture(t, "jsonld_plain.html"))
	if err != nil || d.Title != "Naleśniki" || d.Servings != 8 || len(d.Ingredients) != 3 {
		t.Fatalf("%+v %v", d, err)
	}
	if len(d.Steps) != 3 || d.Steps[2] != "Podawaj z dżemem." {
		t.Errorf("steps=%q", d.Steps)
	}
}

func TestParseHTMLMicrodata(t *testing.T) {
	d, err := ParseHTML(fixture(t, "microdata.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Sałatka grecka" || d.Servings != 2 || len(d.Ingredients) != 3 || len(d.Steps) != 2 {
		t.Fatalf("%+v", d)
	}
	if d.Ingredients[1].Name != "fety" || *d.Ingredients[1].Quantity != 100 {
		t.Errorf("%+v", d.Ingredients[1])
	}
}

func TestParseHTMLFallsBackToPageTextWithAWarning(t *testing.T) {
	d, err := ParseHTML(fixture(t, "plain_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Placki ziemniaczane" || len(d.Ingredients) != 3 || len(d.Steps) != 3 {
		t.Fatalf("%+v", d)
	}
	if len(d.Warnings) == 0 || !strings.Contains(d.Warnings[0], "strukturalne") {
		t.Errorf("warnings=%v", d.Warnings)
	}
	for _, s := range d.Steps {
		if strings.Contains(s, "Regulamin") || strings.Contains(s, "var x") {
			t.Errorf("page chrome leaked into steps: %q", s)
		}
	}
}

func TestParseHTMLRefusesPagesWithoutARecipe(t *testing.T) {
	for _, name := range []string{"menu_only.html"} {
		if _, err := ParseHTML(fixture(t, name)); !errors.Is(err, ErrUnparsable) {
			t.Errorf("%s: expected ErrUnparsable, got %v", name, err)
		}
	}
	for _, in := range []string{"", "<html></html>", "<<<>>>", `<script type="application/ld+json">{broken</script>`} {
		if _, err := ParseHTML([]byte(in)); !errors.Is(err, ErrUnparsable) {
			t.Errorf("%q: expected ErrUnparsable, got %v", in, err)
		}
	}
}

func TestParseHTMLIgnoresEmptyJSONLDRecipe(t *testing.T) {
	page := `<script type="application/ld+json">{"@type":"Recipe","name":"Puste"}</script>` +
		`<div itemscope itemtype="https://schema.org/Recipe"><span itemprop="name">Z microdanych</span>` +
		`<span itemprop="recipeIngredient">1 jajko</span></div>`
	d, err := ParseHTML([]byte(page))
	if err != nil || d.Title != "Z microdanych" {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestParseHTMLJSONLDWrappersAndCandidates(t *testing.T) {
	rec := `{"@type":"Recipe","name":"R","recipeIngredient":["1 jajko"],"recipeInstructions":"Usmaż."}`
	ld := func(body string) string { return `<script type="application/ld+json">` + body + `</script>` }
	cases := map[string]string{
		"mainEntity":         ld(`{"@type":"WebPage","mainEntity":` + rec + `}`),
		"mainEntityOfPage":   ld(`{"@type":"Article","mainEntityOfPage":` + rec + `}`),
		"graph then main":    ld(`{"@graph":[{"@type":"WebSite"}],"mainEntity":` + rec + `}`),
		"bom":                ld("\ufeff" + rec),
		"comment wrapper":    ld("<!--\n" + rec + "\n-->"),
		"cdata wrapper":      ld("//<![CDATA[\n" + rec + "\n//]]>"),
		"empty then full":    ld(`{"@type":"Recipe","name":"Puste"}`) + ld(rec),
		"empty in array too": ld(`[{"@type":"Recipe","name":"Puste"},` + rec + `]`),
	}
	for name, page := range cases {
		d, err := ParseHTML([]byte(page))
		if err != nil || d.Title != "R" || len(d.Ingredients) != 1 || len(d.Steps) != 1 {
			t.Errorf("%s: %+v %v", name, d, err)
		}
	}
}
