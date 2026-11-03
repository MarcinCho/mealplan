package importer

import (
	"errors"
	"strings"
	"testing"

	"mealplan/internal/domain"
)

func TestFinishEnforcesRecipeLimits(t *testing.T) {
	d := Draft{
		Title:     strings.Repeat("ż", 250),
		Servings:  500,
		SourceURL: "ftp://example.com/r",
		Tags:      []string{" Obiad ", "obiad", "", strings.Repeat("a", 41), "Zupa"},
		Steps:     []string{"  ", strings.Repeat("k", 2500), "ok"},
	}
	for i := 0; i < 210; i++ {
		d.Ingredients = append(d.Ingredients, IngredientLine{Raw: "x", Name: "x"})
	}
	d.finish()
	if n := len([]rune(d.Title)); n != 200 {
		t.Errorf("title runes=%d", n)
	}
	if d.Servings != 0 || d.SourceURL != "" {
		t.Errorf("servings=%d url=%q", d.Servings, d.SourceURL)
	}
	if got := strings.Join(d.Tags, ","); got != "obiad,zupa" {
		t.Errorf("tags=%q", got)
	}
	if len(d.Steps) != 2 || len([]rune(d.Steps[0])) != 2000 {
		t.Errorf("steps=%d first=%d", len(d.Steps), len([]rune(d.Steps[0])))
	}
	if len(d.Ingredients) != 200 {
		t.Errorf("ingredients=%d", len(d.Ingredients))
	}
	if len(d.Warnings) < 4 {
		t.Errorf("expected warnings for every cut, got %v", d.Warnings)
	}
	if d.Tags == nil || d.Steps == nil || d.Ingredients == nil || d.Warnings == nil {
		t.Error("slices must be non-nil for JSON")
	}
}

func TestFinishWarnsAboutMissingParts(t *testing.T) {
	d := Draft{Title: "Zupa"}
	d.finish()
	joined := strings.Join(d.Warnings, "|")
	for _, want := range []string{"porcji", "składników", "kroków"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning about %q: %v", want, d.Warnings)
		}
	}
}

func TestInvalidWrapsDomainError(t *testing.T) {
	if err := invalid("bad %s", "x"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("not wrapped: %v", err)
	}
}

func TestFinishTitleAndSourceURL(t *testing.T) {
	d := Draft{}
	d.finish()
	if !strings.Contains(strings.Join(d.Warnings, "|"), "tytułu") {
		t.Errorf("missing title warning: %v", d.Warnings)
	}
	for in, want := range map[string]string{
		"HTTPS://Example.com/r": "HTTPS://Example.com/r", "Http://example.com/": "Http://example.com/",
		"https://": "", "http:///x": "", "javascript:alert(1)": "", "https:example.com": "", "example.com/x": "",
	} {
		d := Draft{Title: "T", SourceURL: in}
		d.finish()
		if d.SourceURL != want {
			t.Errorf("%q: got %q want %q", in, d.SourceURL, want)
		}
	}
}

func TestFinishDropsEmptyIngredientsAndDedupesWarnings(t *testing.T) {
	d := Draft{Title: "T", Ingredients: []IngredientLine{{Raw: "\x00\x01", Name: "\x00"}, {Raw: "ok", Name: "ok"}}}
	d.finish()
	if len(d.Ingredients) != 1 || d.Ingredients[0].Name != "ok" {
		t.Errorf("ingredients=%+v", d.Ingredients)
	}
	before := append([]string(nil), d.Warnings...)
	d.finish()
	if strings.Join(before, "|") != strings.Join(d.Warnings, "|") {
		t.Errorf("finish is not idempotent: %v then %v", before, d.Warnings)
	}
}

func TestCapWarningsAppearOnce(t *testing.T) {
	text := "Składniki\n" + strings.Repeat("200 g mąki\n", 400) + "Przygotowanie\n" + strings.Repeat("1. Zrób coś ważnego\n", 300)
	d, err := importText(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"składniki ponad limit", "kroki ponad limit"} {
		n := 0
		for _, got := range d.Warnings {
			if strings.Contains(got, w) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%q appears %d times: %v", w, n, d.Warnings)
		}
	}
}
