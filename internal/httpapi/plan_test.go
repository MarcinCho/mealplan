package httpapi_test

import "testing"

type entryJSON struct {
	ID         string  `json:"id"`
	Date       string  `json:"date"`
	Slot       string  `json:"slot"`
	RecipeID   *string `json:"recipeId"`
	FreeText   string  `json:"freeText"`
	Servings   int     `json:"servings"`
	CookUserID *string `json:"cookUserId"`
}

type weekJSON struct {
	Week    string      `json:"week"`
	Entries []entryJSON `json:"entries"`
}

func createRecipe(t *testing.T, c *client, title string) recipeJSON {
	t.Helper()
	rec := c.do("POST", "/api/v1/recipes", recipeBody(title))
	if rec.Code != 201 {
		t.Fatalf("create recipe: %d %s", rec.Code, rec.Body)
	}
	var r recipeJSON
	decode(t, rec, &r)
	return r
}

func meID(t *testing.T, c *client) string {
	t.Helper()
	var me struct{ User struct{ ID string } }
	decode(t, c.do("GET", "/api/v1/auth/me", nil), &me)
	return me.User.ID
}

func TestPlanWeekLifecycle(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Zupa")
	me := meID(t, c)

	rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "dinner", "recipeId": r.ID, "cookUserId": me})
	if rec.Code != 201 {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	var added entryJSON
	decode(t, rec, &added)
	if added.Servings != 4 || added.CookUserID == nil || *added.CookUserID != me || added.RecipeID == nil {
		t.Fatalf("added: %+v", added)
	}

	rec = c.do("PUT", "/api/v1/plan/2026-11-02/entries/"+added.ID, map[string]any{"date": "2026-11-04", "slot": "lunch", "freeText": "Resztki", "servings": 2})
	if rec.Code != 200 {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}

	var wk weekJSON
	decode(t, c.do("GET", "/api/v1/plan/2026-11-02", nil), &wk)
	if wk.Week != "2026-11-02" || len(wk.Entries) != 1 || wk.Entries[0].FreeText != "Resztki" || wk.Entries[0].RecipeID != nil || wk.Entries[0].CookUserID != nil {
		t.Fatalf("week: %+v", wk)
	}

	rec = c.do("POST", "/api/v1/plan/2026-11-09/copy", map[string]any{"fromWeek": "2026-11-02"})
	var cp struct{ Copied int }
	decode(t, rec, &cp)
	if rec.Code != 200 || cp.Copied != 1 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/plan/2026-11-09/copy", map[string]any{"fromWeek": "2026-11-02"}); rec.Code != 409 {
		t.Fatalf("second copy: %d", rec.Code)
	}

	if rec := c.do("DELETE", "/api/v1/plan/2026-11-02/entries/"+added.ID, nil); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := c.do("DELETE", "/api/v1/plan/2026-11-02/entries/"+added.ID, nil); rec.Code != 404 {
		t.Fatalf("double delete: %d", rec.Code)
	}
}

func TestPlanRejectsBadWeeksAndDates(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	for _, wk := range []string{"2026-11-03", "garbage", "2026-02-30"} {
		if rec := c.do("GET", "/api/v1/plan/"+wk, nil); rec.Code != 422 || errCode(t, rec) != "invalid_input" {
			t.Errorf("week %q: %d %s", wk, rec.Code, rec.Body)
		}
	}
	body := map[string]any{"date": "2026-11-20", "slot": "lunch", "freeText": "x"}
	if rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", body); rec.Code != 422 {
		t.Errorf("date outside week: %d", rec.Code)
	}
	if rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "freeText": "x", "servings": 0, "extra": 1}); rec.Code != 400 {
		t.Errorf("unknown field: %d", rec.Code)
	}
	if rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "freeText": "x", "servings": -5}); rec.Code != 422 {
		t.Errorf("negative servings: %d", rec.Code)
	}
}

func TestPlanIsolationAndForeignReferences(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	ra := createRecipe(t, a, "Tajny przepis A")
	var added entryJSON
	decode(t, a.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "freeText": "A"}), &added)

	b := e.secondHouseholdClient(t)
	bID := meID(t, b)

	var wk weekJSON
	decode(t, b.do("GET", "/api/v1/plan/2026-11-02", nil), &wk)
	if len(wk.Entries) != 0 {
		t.Fatalf("foreign week leaks: %+v", wk)
	}
	if rec := b.do("PUT", "/api/v1/plan/2026-11-02/entries/"+added.ID, map[string]any{"date": "2026-11-03", "slot": "lunch", "freeText": "hacked"}); rec.Code != 404 {
		t.Errorf("edit foreign entry: %d", rec.Code)
	}
	if rec := b.do("DELETE", "/api/v1/plan/2026-11-02/entries/"+added.ID, nil); rec.Code != 404 {
		t.Errorf("delete foreign entry: %d", rec.Code)
	}
	if rec := b.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "recipeId": ra.ID}); rec.Code != 404 {
		t.Errorf("foreign recipe: %d", rec.Code)
	}
	aID := meID(t, a)
	if rec := b.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "freeText": "x", "cookUserId": aID}); rec.Code != 404 {
		t.Errorf("foreign cook: %d", rec.Code)
	}
	if rec := b.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "freeText": "ok", "cookUserId": bID}); rec.Code != 201 {
		t.Errorf("own cook: %d", rec.Code)
	}
	decode(t, a.do("GET", "/api/v1/plan/2026-11-02", nil), &wk)
	if len(wk.Entries) != 1 || wk.Entries[0].FreeText != "A" {
		t.Fatalf("victim week changed: %+v", wk)
	}
}

func TestRecipeInUseCannotBeDeleted(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Zupa")
	c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "lunch", "recipeId": r.ID})
	if rec := c.do("DELETE", "/api/v1/recipes/"+r.ID, nil); rec.Code != 409 || errCode(t, rec) != "in_use" {
		t.Fatalf("delete used recipe: %d %s", rec.Code, rec.Body)
	}
}
