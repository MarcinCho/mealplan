package recipe_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/recipe"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func newSvc(t *testing.T) (*recipe.Service, string) {
	t.Helper()
	st := store.New(storetest.Open(t))
	err := st.CreateHouseholdWithAdmin(context.Background(),
		domain.Household{ID: "h1", Name: "Dom"}, domain.User{ID: "u1", Login: "marc", PasswordHash: "x", DisplayName: "M"}, false)
	if err != nil {
		t.Fatal(err)
	}
	return recipe.New(st, func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) }), "h1"
}

func q(v float64) *float64 { return &v }

func valid() recipe.Input {
	return recipe.Input{
		Title: "  Zupa  ", BaseServings: 4, SourceURL: "https://example.com/z",
		Tags:        []string{" Szybkie ", "szybkie", "OBIAD"},
		Ingredients: []recipe.IngredientInput{{Name: " Marchew ", Quantity: q(2), Unit: "Szt."}, {Name: "Sól"}},
		Steps:       []string{" Gotuj. "},
	}
}

func TestCreateNormalizesInput(t *testing.T) {
	svc, hid := newSvc(t)
	r, err := svc.Create(context.Background(), hid, valid())
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Zupa" || len(r.Tags) != 2 || r.Tags[0] != "obiad" && r.Tags[1] != "obiad" {
		t.Fatalf("title/tags: %+v", r)
	}
	ing := r.Ingredients[0]
	if ing.Name != "Marchew" || ing.NormalizedName != "marchew" || ing.Unit != "szt" {
		t.Fatalf("ingredient: %+v", ing)
	}
	if r.Steps[0] != "Gotuj." || r.ID == "" || r.HouseholdID != hid {
		t.Fatalf("recipe: %+v", r)
	}
	got, err := svc.Get(context.Background(), hid, r.ID)
	if err != nil || got.Title != "Zupa" {
		t.Fatalf("get: %v %+v", err, got)
	}
}

func TestValidationRejects(t *testing.T) {
	svc, hid := newSvc(t)
	mutate := map[string]func(*recipe.Input){
		"empty title":       func(i *recipe.Input) { i.Title = "  " },
		"long title":        func(i *recipe.Input) { i.Title = strings.Repeat("a", 201) },
		"zero servings":     func(i *recipe.Input) { i.BaseServings = 0 },
		"negative servings": func(i *recipe.Input) { i.BaseServings = -2 },
		"huge servings":     func(i *recipe.Input) { i.BaseServings = 101 },
		"js url":            func(i *recipe.Input) { i.SourceURL = "javascript:alert(1)" },
		"ftp url":           func(i *recipe.Input) { i.SourceURL = "ftp://x/y" },
		"empty ingredient":  func(i *recipe.Input) { i.Ingredients[0].Name = " " },
		"negative quantity": func(i *recipe.Input) { i.Ingredients[0].Quantity = q(-1) },
		"zero quantity":     func(i *recipe.Input) { i.Ingredients[0].Quantity = q(0) },
		"giant quantity":    func(i *recipe.Input) { i.Ingredients[0].Quantity = q(1e9) },
		"long unit":         func(i *recipe.Input) { i.Ingredients[0].Unit = strings.Repeat("u", 31) },
		"empty step":        func(i *recipe.Input) { i.Steps = []string{"  "} },
		"too many tags": func(i *recipe.Input) {
			i.Tags = nil
			for n := 0; n < 21; n++ {
				i.Tags = append(i.Tags, string(rune('a'+n)))
			}
		},
		"long tag": func(i *recipe.Input) { i.Tags = []string{strings.Repeat("t", 41)} },
		"too many ingredients": func(i *recipe.Input) {
			i.Ingredients = make([]recipe.IngredientInput, 201)
			for n := range i.Ingredients {
				i.Ingredients[n] = recipe.IngredientInput{Name: "x"}
			}
		},
		"too many steps": func(i *recipe.Input) {
			i.Steps = make([]string, 101)
			for n := range i.Steps {
				i.Steps[n] = "s"
			}
		},
	}
	for name, m := range mutate {
		in := valid()
		m(&in)
		if _, err := svc.Create(context.Background(), hid, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: expected ErrInvalid, got %v", name, err)
		}
	}
}

func TestUpdateAndDeleteScopedToHousehold(t *testing.T) {
	svc, hid := newSvc(t)
	r, _ := svc.Create(context.Background(), hid, valid())
	if _, err := svc.Update(context.Background(), "other", r.ID, valid()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update foreign: %v", err)
	}
	in := valid()
	in.Title = "Rosół"
	up, err := svc.Update(context.Background(), hid, r.ID, in)
	if err != nil || up.Title != "Rosół" || !up.CreatedAt.Equal(r.CreatedAt) {
		t.Fatalf("update: %v %+v", err, up)
	}
	if err := svc.Delete(context.Background(), "other", r.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete foreign: %v", err)
	}
}

func TestRejectsControlCharacters(t *testing.T) {
	svc, hid := newSvc(t)
	cases := map[string]func(in *recipe.Input){
		"title":           func(in *recipe.Input) { in.Title = "Zu\x00pa" },
		"ingredient name": func(in *recipe.Input) { in.Ingredients[0].Name = "Mar\x00chew" },
		"ingredient unit": func(in *recipe.Input) { in.Ingredients[0].Unit = "sz\x00t" },
		"step":            func(in *recipe.Input) { in.Steps[0] = "Got\x00uj" },
		"tag":             func(in *recipe.Input) { in.Tags[0] = "ob\x00iad" },
		"sourceUrl":       func(in *recipe.Input) { in.SourceURL = "https://example.com/\x00" },
		"bell in title":   func(in *recipe.Input) { in.Title = "Zu\x07pa" },
	}
	for name, mut := range cases {
		in := valid()
		mut(&in)
		if _, err := svc.Create(context.Background(), hid, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: expected ErrInvalid, got %v", name, err)
		}
	}
	in := valid()
	in.Steps = []string{"Linia 1\n\tLinia 2\r\nLinia 3"}
	if _, err := svc.Create(context.Background(), hid, in); err != nil {
		t.Fatalf("newlines and tabs in steps must stay allowed: %v", err)
	}
}
