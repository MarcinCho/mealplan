package importer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/importer"
	"mealplan/internal/recipe"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func accepted(t *testing.T, d importer.Draft) {
	t.Helper()
	importer.Finish(&d)
	st := store.New(storetest.Open(t))
	err := st.CreateHouseholdWithAdmin(context.Background(),
		domain.Household{ID: "h1", Name: "Dom"}, domain.User{ID: "u1", Login: "marc", PasswordHash: "x", DisplayName: "M"}, false)
	if err != nil {
		t.Fatal(err)
	}
	svc := recipe.New(st, time.Now)
	in := recipe.Input{Title: d.Title, BaseServings: 4, SourceURL: d.SourceURL, Tags: d.Tags, Steps: d.Steps}
	for _, i := range d.Ingredients {
		in.Ingredients = append(in.Ingredients, recipe.IngredientInput{Name: i.Name, Quantity: i.Quantity, Unit: i.Unit, Note: i.Note})
		for _, s := range []string{i.Raw, i.Name, i.Note, i.Unit} {
			if domain.HasControl(s) {
				t.Errorf("control chars left in %q", s)
			}
		}
	}
	if _, err := svc.Create(context.Background(), "h1", in); err != nil {
		t.Fatalf("draft rejected: %v (%+v)", err, d)
	}
}

func TestJSONLDWithControlCharsIsSavable(t *testing.T) {
	page := `<script type="application/ld+json">{"@type":"Recipe","name":"Ciasto\u0007 z\u0000 jabłkami","keywords":"a\u0001b",` +
		`"recipeIngredient":["2\u000b jajka","200 g\u0000 mąki"],"recipeInstructions":["Wymieszaj\u0007  całość\u007f."]}</script>`
	d, err := importer.ParseHTML([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	accepted(t, d)
}

func TestPastedTextWithControlCharsIsSavable(t *testing.T) {
	svc := importer.New()
	d, err := svc.Import(context.Background(), importer.Source{Kind: importer.KindText,
		Text: "Ciasto\f\v drożdżowe\nSkładniki\n2\v jajka\n200 g\f mąki\nPrzygotowanie\n1. Wymieszaj\f całość."})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(d.Title, "\f\v") {
		t.Errorf("title=%q", d.Title)
	}
	accepted(t, d)
}

func TestDraftWithSavedNotePassesRecipeValidation(t *testing.T) {
	d, err := importer.New().Import(context.Background(), importer.Source{Kind: importer.KindText,
		Text: "Sernik\nSkładniki\n250 g jogurt grecki (można zastąpić kefirem lub śmietaną 18%)\n2 jajka\nPrzygotowanie\n1. Wymieszaj."})
	if err != nil {
		t.Fatal(err)
	}
	if d.Ingredients[0].Note != "można zastąpić kefirem lub śmietaną 18%" {
		t.Fatalf("draft: %+v", d.Ingredients)
	}
	accepted(t, d)
}
