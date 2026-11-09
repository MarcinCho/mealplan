package httpapi_test

import (
	"net/http/httptest"
	"testing"
)

type itemJSON struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Quantity *float64 `json:"quantity"`
	Unit     string   `json:"unit"`
	Checked  bool     `json:"checked"`
	Manual   bool     `json:"manual"`
}

type listJSON struct {
	Week               string     `json:"week"`
	Items              []itemJSON `json:"items"`
	PreviousWeekUndone *int       `json:"previousWeekUndone"`
}

func TestShoppingFlowOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Naleśniki") // 250 g mąki + sól, 4 porcje
	c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "dinner", "recipeId": r.ID, "servings": 8})

	rec := c.do("POST", "/api/v1/shopping/generate", map[string]any{"week": "2026-11-02"})
	var gen listJSON
	decode(t, rec, &gen)
	if rec.Code != 200 || len(gen.Items) != 2 {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body)
	}
	var flour *itemJSON
	for i := range gen.Items {
		if gen.Items[i].Name == "Mąka" {
			flour = &gen.Items[i]
		}
	}
	if flour == nil || flour.Quantity == nil || *flour.Quantity != 500 || flour.Unit != "g" {
		t.Fatalf("flour: %+v", gen.Items)
	}

	if rec := c.do("PATCH", "/api/v1/shopping/items/"+flour.ID, map[string]any{"checked": true}); rec.Code != 204 {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	rec = c.do("POST", "/api/v1/shopping/items", map[string]any{"week": "2026-11-02", "name": "Papier", "quantity": 2, "unit": "szt"})
	if rec.Code != 201 {
		t.Fatalf("manual: %d %s", rec.Code, rec.Body)
	}
	var manual itemJSON
	decode(t, rec, &manual)

	var list listJSON
	decode(t, c.do("GET", "/api/v1/shopping?week=2026-11-02", nil), &list)
	if len(list.Items) != 3 || list.Items[len(list.Items)-1].ID != flour.ID {
		t.Fatalf("checked item must sort last: %+v", list.Items)
	}

	if rec := c.do("PATCH", "/api/v1/shopping/items/"+manual.ID, map[string]any{"name": "Chleb", "quantity": 1, "unit": "szt"}); rec.Code != 204 {
		t.Fatalf("edit manual: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("PATCH", "/api/v1/shopping/items/"+flour.ID, map[string]any{"name": "Inna"}); rec.Code != 422 {
		t.Fatalf("edit generated: %d", rec.Code)
	}
	if rec := c.do("DELETE", "/api/v1/shopping/items/"+manual.ID, nil); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestShoppingValidation(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	if rec := c.do("GET", "/api/v1/shopping", nil); rec.Code != 422 {
		t.Errorf("missing week: %d", rec.Code)
	}
	if rec := c.do("GET", "/api/v1/shopping?week=2026-11-03", nil); rec.Code != 422 {
		t.Errorf("non-monday: %d", rec.Code)
	}
	if rec := c.do("POST", "/api/v1/shopping/generate", map[string]any{"week": "2026-11-02", "from": "2026-11-05"}); rec.Code != 422 {
		t.Errorf("half range: %d", rec.Code)
	}
	if rec := c.do("POST", "/api/v1/shopping/items", map[string]any{"week": "2026-11-02", "name": ""}); rec.Code != 422 {
		t.Errorf("empty name: %d", rec.Code)
	}
	if rec := c.do("PATCH", "/api/v1/shopping/items/nope", map[string]any{"checked": true}); rec.Code != 404 {
		t.Errorf("unknown item: %d", rec.Code)
	}
	if rec := c.do("PATCH", "/api/v1/shopping/items/nope", map[string]any{}); rec.Code != 422 {
		t.Errorf("empty patch: %d", rec.Code)
	}
}

func TestShoppingIsolation(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	var item itemJSON
	decode(t, a.do("POST", "/api/v1/shopping/items", map[string]any{"week": "2026-11-02", "name": "Tajne A"}), &item)

	b := e.secondHouseholdClient(t)
	var list listJSON
	decode(t, b.do("GET", "/api/v1/shopping?week=2026-11-02", nil), &list)
	if len(list.Items) != 0 {
		t.Fatalf("leak: %+v", list)
	}
	if rec := b.do("PATCH", "/api/v1/shopping/items/"+item.ID, map[string]any{"checked": true}); rec.Code != 404 {
		t.Errorf("check foreign: %d", rec.Code)
	}
	if rec := b.do("DELETE", "/api/v1/shopping/items/"+item.ID, nil); rec.Code != 404 {
		t.Errorf("delete foreign: %d", rec.Code)
	}
	decode(t, a.do("GET", "/api/v1/shopping?week=2026-11-02", nil), &list)
	if len(list.Items) != 1 || list.Items[0].Checked {
		t.Fatalf("victim changed: %+v", list)
	}
}

func TestShoppingListReportsPreviousWeekUndone(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Naleśniki")
	c.do("POST", "/api/v1/plan/2026-10-26/entries", map[string]any{"date": "2026-10-27", "slot": "dinner", "recipeId": r.ID})
	c.do("POST", "/api/v1/plan/2026-10-26/entries", map[string]any{"date": "2026-10-28", "slot": "dinner", "freeText": "pizza"})
	for _, call := range []func() *httptest.ResponseRecorder{
		func() *httptest.ResponseRecorder { return c.do("GET", "/api/v1/shopping?week=2026-11-02", nil) },
		func() *httptest.ResponseRecorder {
			return c.do("POST", "/api/v1/shopping/generate", map[string]any{"week": "2026-11-02"})
		},
	} {
		rec := call()
		var l listJSON
		decode(t, rec, &l)
		if rec.Code != 200 || l.PreviousWeekUndone == nil || *l.PreviousWeekUndone != 1 {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
	}
}
