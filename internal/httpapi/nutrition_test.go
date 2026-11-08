package httpapi_test

import "testing"

type macrosOut struct {
	Kcal, Protein, Carbs, Fat *float64
}

type dayOut struct {
	Date      string
	Total     macrosOut
	Goal      macrosOut
	Remaining macrosOut
	Meals     []struct {
		EntryID, Title, Slot string
		Values               macrosOut
		Missing              bool
	}
	Log []struct {
		ID, ProductID, Name, Unit string
		Quantity                  float64
		Values                    macrosOut
		Missing                   bool
	}
	MissingMeals int
	MissingLog   int
	Incomplete   struct{ Kcal, Protein, Carbs, Fat bool }
	Approximate  bool
}

type daysOut struct {
	Enabled bool
	Days    []dayOut
}

func createProduct(t *testing.T, c *client, name string) string {
	t.Helper()
	rec := c.do("POST", "/api/v1/products", map[string]any{"name": name, "per100g": map[string]any{"kcal": 60, "protein": 4, "carbs": nil, "fat": nil}, "aliases": []string{}})
	if rec.Code != 201 {
		t.Fatalf("create product: %d %s", rec.Code, rec.Body)
	}
	var p struct{ ID string }
	decode(t, rec, &p)
	return p.ID
}

func TestNutritionSettingsAndDays(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	var set struct {
		Enabled bool
		Goal    macrosOut
	}
	decode(t, c.do("GET", "/api/v1/me/nutrition", nil), &set)
	if set.Enabled || set.Goal.Kcal != nil {
		t.Fatalf("default: %+v", set)
	}
	var off daysOut
	rec := c.do("GET", "/api/v1/me/nutrition/days?from=2026-11-03&to=2026-11-03", nil)
	decode(t, rec, &off)
	if rec.Code != 200 || off.Enabled || off.Days == nil || len(off.Days) != 0 {
		t.Fatalf("disabled: %d %s", rec.Code, rec.Body)
	}

	if rec := c.do("PUT", "/api/v1/me/nutrition", map[string]any{"enabled": true, "goal": map[string]any{"kcal": 25000}}); rec.Code != 422 || errCode(t, rec) != "invalid_input" {
		t.Fatalf("bad goal: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("PUT", "/api/v1/me/nutrition", map[string]any{"enabled": true, "goal": map[string]any{"kcal": 2000, "protein": 100, "carbs": nil, "fat": nil}}); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	decode(t, c.do("GET", "/api/v1/me/nutrition", nil), &set)
	if !set.Enabled || *set.Goal.Kcal != 2000 {
		t.Fatalf("saved: %+v", set)
	}

	pid := createProduct(t, c, "Jogurt domowy")
	rec = c.do("POST", "/api/v1/me/food-log", map[string]any{"date": "2026-11-03", "productId": pid, "quantity": 150, "unit": "g"})
	if rec.Code != 201 {
		t.Fatalf("log: %d %s", rec.Code, rec.Body)
	}
	var logged struct{ ID string }
	decode(t, rec, &logged)

	r := createRecipe(t, c, "Zupa")
	if rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "dinner", "recipeId": r.ID}); rec.Code != 201 {
		t.Fatalf("plan: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "snack", "freeText": "Owoce"}); rec.Code != 201 {
		t.Fatalf("plan: %d %s", rec.Code, rec.Body)
	}

	rec = c.do("GET", "/api/v1/me/nutrition/days?from=2026-11-02&to=2026-11-04", nil)
	var days daysOut
	decode(t, rec, &days)
	if !days.Enabled || len(days.Days) != 3 {
		t.Fatalf("days: %s", rec.Body)
	}
	d := days.Days[1]
	if d.Date != "2026-11-03" || len(d.Meals) != 2 || len(d.Log) != 1 || d.MissingMeals != 1 || *d.Log[0].Values.Kcal != 90 || *d.Total.Kcal != 317.5 || *d.Remaining.Kcal != 1682.5 || d.Remaining.Carbs != nil {
		t.Fatalf("day: %+v", d)
	}
	if !d.Incomplete.Carbs || !d.Approximate || d.MissingLog != 0 {
		t.Fatalf("flags: %+v", d)
	}
	if d.Log[0].Name != "Jogurt domowy" || d.Log[0].Values.Carbs != nil {
		t.Fatalf("log: %+v", d.Log[0])
	}
	if e0 := days.Days[0]; e0.Meals == nil || e0.Log == nil {
		t.Fatalf("empty arrays must not be null: %s", rec.Body)
	}

	if rec := c.do("DELETE", "/api/v1/me/food-log/"+logged.ID, nil); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := c.do("DELETE", "/api/v1/me/food-log/"+logged.ID, nil); rec.Code != 404 {
		t.Fatalf("delete again: %d", rec.Code)
	}
}

func TestNutritionValidationAndIsolationOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	b := e.secondHouseholdClient(t)
	pid := createProduct(t, c, "Jogurt rodzinny")
	foreign := createProduct(t, b, "Obcy produkt")

	for name, body := range map[string]map[string]any{
		"unit":     {"date": "2026-11-03", "productId": pid, "quantity": 1, "unit": "kg"},
		"quantity": {"date": "2026-11-03", "productId": pid, "quantity": 0, "unit": "g"},
		"big":      {"date": "2026-11-03", "productId": pid, "quantity": 10001, "unit": "g"},
		"date":     {"date": "03.11.2026", "productId": pid, "quantity": 1, "unit": "g"},
	} {
		if rec := c.do("POST", "/api/v1/me/food-log", body); rec.Code != 422 {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if rec := c.do("POST", "/api/v1/me/food-log", map[string]any{"date": "2026-11-03", "productId": foreign, "quantity": 1, "unit": "g"}); rec.Code != 404 {
		t.Fatalf("foreign product: %d %s", rec.Code, rec.Body)
	}
	rec := c.do("POST", "/api/v1/me/food-log", map[string]any{"date": "2026-11-03", "productId": pid, "quantity": 1, "unit": "g"})
	var logged struct{ ID string }
	decode(t, rec, &logged)
	if rec := b.do("DELETE", "/api/v1/me/food-log/"+logged.ID, nil); rec.Code != 404 {
		t.Fatalf("foreign delete: %d", rec.Code)
	}
	c.do("PUT", "/api/v1/me/nutrition", map[string]any{"enabled": true, "goal": map[string]any{}})
	b.do("PUT", "/api/v1/me/nutrition", map[string]any{"enabled": true, "goal": map[string]any{}})
	var days daysOut
	decode(t, b.do("GET", "/api/v1/me/nutrition/days?from=2026-11-03&to=2026-11-03", nil), &days)
	if len(days.Days) != 1 || len(days.Days[0].Log) != 0 {
		t.Fatalf("log leaked: %+v", days)
	}
	for _, q := range []string{"from=2026-11-04&to=2026-11-03", "from=2026-11-01&to=2026-12-02", "from=x&to=2026-11-03", "to=2026-11-03", ""} {
		if rec := c.do("GET", "/api/v1/me/nutrition/days?"+q, nil); rec.Code != 422 {
			t.Fatalf("%q: %d %s", q, rec.Code, rec.Body)
		}
	}
	if rec := newEnv(t).client(t).do("GET", "/api/v1/me/nutrition", nil); rec.Code != 401 {
		t.Fatalf("anon: %d", rec.Code)
	}
}
