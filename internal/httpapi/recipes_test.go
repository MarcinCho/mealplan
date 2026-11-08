package httpapi_test

import (
	"strings"
	"testing"
)

type recipeJSON struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	BaseServings int      `json:"baseServings"`
	SourceURL    string   `json:"sourceUrl"`
	Tags         []string `json:"tags"`
	Ingredients  []struct {
		Name     string   `json:"name"`
		Quantity *float64 `json:"quantity"`
		Unit     string   `json:"unit"`
		Note     string   `json:"note"`
	} `json:"ingredients"`
	Steps []string `json:"steps"`
}

func recipeBody(title string) map[string]any {
	return map[string]any{
		"title": title, "baseServings": 4, "sourceUrl": "https://example.com/r", "tags": []string{"obiad"},
		"ingredients": []map[string]any{{"name": "Mąka", "quantity": 250, "unit": "g"}, {"name": "Sól", "quantity": nil, "unit": ""}},
		"steps":       []string{"Wymieszaj."},
	}
}

func TestRecipeCRUDOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	rec := c.do("POST", "/api/v1/recipes", recipeBody("Naleśniki"))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created recipeJSON
	decode(t, rec, &created)
	if created.ID == "" || created.Ingredients[1].Quantity != nil {
		t.Fatalf("created: %+v", created)
	}

	var got recipeJSON
	decode(t, c.do("GET", "/api/v1/recipes/"+created.ID, nil), &got)
	if got.Title != "Naleśniki" || len(got.Steps) != 1 {
		t.Fatalf("get: %+v", got)
	}

	rec = c.do("PUT", "/api/v1/recipes/"+created.ID, recipeBody("Racuchy"))
	if rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}

	var list []recipeJSON
	decode(t, c.do("GET", "/api/v1/recipes", nil), &list)
	if len(list) != 1 || list[0].Title != "Racuchy" || list[0].Ingredients == nil || len(list[0].Ingredients) != 0 {
		t.Fatalf("list: %+v", list)
	}

	if rec := c.do("DELETE", "/api/v1/recipes/"+created.ID, nil); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("GET", "/api/v1/recipes/"+created.ID, nil); rec.Code != 404 {
		t.Fatalf("after delete: %d", rec.Code)
	}
}

func TestRecipeValidationAndAuth(t *testing.T) {
	e := newEnv(t)
	anon := e.client(t)
	if rec := anon.do("GET", "/api/v1/recipes", nil); rec.Code != 401 {
		t.Fatalf("anonymous list: %d", rec.Code)
	}
	c := e.client(t)
	setup(t, c)

	bad := recipeBody("")
	if rec := c.do("POST", "/api/v1/recipes", bad); rec.Code != 422 || errCode(t, rec) != "invalid_input" {
		t.Fatalf("empty title: %d %s", rec.Code, rec.Body)
	}
	js := recipeBody("X")
	js["sourceUrl"] = "javascript:alert(1)"
	if rec := c.do("POST", "/api/v1/recipes", js); rec.Code != 422 {
		t.Fatalf("js url: %d", rec.Code)
	}
	if rec := c.do("POST", "/api/v1/recipes", `{"title":"x","baseServings":2,"bogus":1}`); rec.Code != 400 {
		t.Fatalf("unknown field: %d", rec.Code)
	}
	noCSRF := *c
	noCSRF.csrf = ""
	if rec := noCSRF.do("POST", "/api/v1/recipes", recipeBody("X")); rec.Code != 403 {
		t.Fatalf("missing csrf: %d", rec.Code)
	}
	if rec := c.do("GET", "/api/v1/recipes/"+strings.Repeat("0", 32), nil); rec.Code != 404 {
		t.Fatalf("unknown id: %d", rec.Code)
	}
}

func TestRecipesAreIsolatedBetweenHouseholds(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	var created recipeJSON
	decode(t, a.do("POST", "/api/v1/recipes", recipeBody("Sekret A")), &created)

	b := e.secondHouseholdClient(t)
	if rec := b.do("GET", "/api/v1/recipes/"+created.ID, nil); rec.Code != 404 {
		t.Fatalf("read foreign: %d", rec.Code)
	}
	if rec := b.do("PUT", "/api/v1/recipes/"+created.ID, recipeBody("Hacked")); rec.Code != 404 {
		t.Fatalf("update foreign: %d", rec.Code)
	}
	if rec := b.do("DELETE", "/api/v1/recipes/"+created.ID, nil); rec.Code != 404 {
		t.Fatalf("delete foreign: %d", rec.Code)
	}
	var list []recipeJSON
	decode(t, b.do("GET", "/api/v1/recipes", nil), &list)
	if len(list) != 0 {
		t.Fatalf("foreign list leaks: %+v", list)
	}
	var mine recipeJSON
	decode(t, a.do("GET", "/api/v1/recipes/"+created.ID, nil), &mine)
	if mine.Title != "Sekret A" {
		t.Fatalf("victim changed: %+v", mine)
	}
}

func TestRecipeIngredientNoteOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	body := recipeBody("Sernik")
	body["ingredients"] = []map[string]any{
		{"name": "Jogurt", "quantity": 250, "unit": "g", "note": "można zastąpić jogurtem greckim"},
		{"name": "Sól", "quantity": nil, "unit": ""},
	}
	rec := c.do("POST", "/api/v1/recipes", body)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created recipeJSON
	decode(t, rec, &created)
	if created.Ingredients[0].Note != "można zastąpić jogurtem greckim" || created.Ingredients[1].Note != "" {
		t.Fatalf("created: %+v", created.Ingredients)
	}
	var got recipeJSON
	decode(t, c.do("GET", "/api/v1/recipes/"+created.ID, nil), &got)
	if got.Ingredients[0].Note != "można zastąpić jogurtem greckim" || got.Ingredients[1].Note != "" {
		t.Fatalf("get: %+v", got.Ingredients)
	}

	body["ingredients"] = []map[string]any{{"name": "Jogurt", "quantity": 250, "unit": "g", "note": "kefir"}}
	if rec := c.do("PUT", "/api/v1/recipes/"+created.ID, body); rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	decode(t, c.do("GET", "/api/v1/recipes/"+created.ID, nil), &got)
	if got.Ingredients[0].Note != "kefir" {
		t.Fatalf("after update: %+v", got.Ingredients)
	}

	body["ingredients"] = []map[string]any{{"name": "Jogurt", "quantity": 250, "unit": "g", "note": strings.Repeat("ż", 201)}}
	if rec := c.do("POST", "/api/v1/recipes", body); rec.Code != 422 || !strings.Contains(rec.Body.String(), "invalid_input") {
		t.Fatalf("long note: %d %s", rec.Code, rec.Body)
	}
}

type nutritionResp struct {
	Nutrition struct {
		Kcal, Protein, Carbs, Fat *float64
	} `json:"nutrition"`
	Ingredients []struct {
		ProductID *string `json:"productId"`
	} `json:"ingredients"`
	NutritionPerPortion *struct {
		Values struct {
			Kcal, Protein, Carbs, Fat *float64
		} `json:"values"`
		Sources struct {
			Kcal, Protein, Carbs, Fat string
		} `json:"sources"`
		Approximate bool     `json:"approximate"`
		Unmatched   []string `json:"unmatched"`
	} `json:"nutritionPerPortion"`
}

func TestRecipeNutritionOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	var prod productJSON
	pb := map[string]any{"name": "Mąka testowa", "per100g": map[string]any{"kcal": 360, "protein": 10, "carbs": 70, "fat": nil}, "aliases": []string{}}
	decode(t, c.do("POST", "/api/v1/products", pb), &prod)

	body := recipeBody("Placek")
	body["nutrition"] = map[string]any{"kcal": nil, "protein": 7, "carbs": nil, "fat": nil}
	body["ingredients"] = []map[string]any{
		{"name": "coś dziwnego", "quantity": 200, "unit": "g", "productId": prod.ID},
		{"name": "Sól", "quantity": nil, "unit": ""},
	}
	rec := c.do("POST", "/api/v1/recipes", body)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created recipeJSON
	decode(t, rec, &created)

	var got nutritionResp
	decode(t, c.do("GET", "/api/v1/recipes/"+created.ID, nil), &got)
	if got.Nutrition.Protein == nil || *got.Nutrition.Protein != 7 || got.Nutrition.Kcal != nil {
		t.Fatalf("given: %+v", got.Nutrition)
	}
	if got.Ingredients[0].ProductID == nil || *got.Ingredients[0].ProductID != prod.ID || got.Ingredients[1].ProductID != nil {
		t.Fatalf("productId: %+v", got.Ingredients)
	}
	n := got.NutritionPerPortion
	if n == nil || n.Values.Kcal == nil || *n.Values.Kcal != 180 || *n.Values.Protein != 7 || *n.Values.Carbs != 35 || n.Values.Fat != nil {
		t.Fatalf("per portion: %+v", n)
	}
	if n.Sources.Kcal != "computed" || n.Sources.Protein != "recipe" || n.Sources.Carbs != "computed" || n.Sources.Fat != "missing" {
		t.Fatalf("sources: %+v", n.Sources)
	}
	if n.Approximate || n.Unmatched == nil || len(n.Unmatched) != 0 {
		t.Fatalf("approximate/unmatched: %+v", n)
	}
	if !strings.Contains(c.do("GET", "/api/v1/recipes/"+created.ID, nil).Body.String(), `"fat":null`) {
		t.Fatal("unknown values must be JSON null")
	}

	// List does not compute nutrition.
	if strings.Contains(c.do("GET", "/api/v1/recipes", nil).Body.String(), "nutritionPerPortion") {
		t.Fatal("list must not include nutritionPerPortion")
	}

	// Old clients: no nutrition, no productId.
	old := c.do("POST", "/api/v1/recipes", recipeBody("Stary"))
	if old.Code != 201 {
		t.Fatalf("old client: %d %s", old.Code, old.Body)
	}
	var oc nutritionResp
	decode(t, old, &oc)
	if oc.Nutrition.Kcal != nil || oc.Nutrition.Protein != nil || oc.Nutrition.Carbs != nil || oc.Nutrition.Fat != nil {
		t.Fatalf("old nutrition: %+v", oc.Nutrition)
	}

	// Out of range, and a foreign product.
	body["nutrition"] = map[string]any{"kcal": 6000}
	if rec := c.do("PUT", "/api/v1/recipes/"+created.ID, body); rec.Code != 422 {
		t.Fatalf("range: %d %s", rec.Code, rec.Body)
	}
	body["nutrition"] = nil
	other := e.secondHouseholdClient(t)
	var fp productJSON
	decode(t, other.do("POST", "/api/v1/products", pb), &fp)
	body["ingredients"] = []map[string]any{{"name": "X", "quantity": 1, "unit": "g", "productId": fp.ID}}
	if rec := c.do("POST", "/api/v1/recipes", body); rec.Code != 404 {
		t.Fatalf("foreign product: %d %s", rec.Code, rec.Body)
	}
}
