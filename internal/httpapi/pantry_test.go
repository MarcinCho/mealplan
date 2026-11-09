package httpapi_test

import "testing"

type pantryItemJSON struct {
	ID        string   `json:"id"`
	ProductID string   `json:"productId"`
	Name      string   `json:"name"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
	ExpiresOn string   `json:"expiresOn"`
	Staple    bool     `json:"staple"`
}

type pantryListJSON struct {
	Items      []pantryItemJSON `json:"items"`
	ReviewedAt *string          `json:"reviewedAt"`
}

func TestPantryCRUDAndMerge(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	var empty pantryListJSON
	decode(t, c.do("GET", "/api/v1/pantry", nil), &empty)
	if empty.Items == nil || len(empty.Items) != 0 || empty.ReviewedAt != nil {
		t.Fatalf("empty: %+v", empty)
	}

	rec := c.do("POST", "/api/v1/pantry", map[string]any{"name": "Mleko", "quantity": 500, "unit": "ml", "expiresOn": "2026-11-20"})
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var a pantryItemJSON
	decode(t, rec, &a)
	if a.ID == "" || a.ProductID == "" || a.Name != "mleko" || *a.Quantity != 500 || a.ExpiresOn != "2026-11-20" {
		t.Fatalf("created: %+v", a)
	}

	rec = c.do("POST", "/api/v1/pantry", map[string]any{"productId": a.ProductID, "quantity": 250, "unit": "ml"})
	var b pantryItemJSON
	decode(t, rec, &b)
	if rec.Code != 201 || b.ID != a.ID || *b.Quantity != 750 {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body)
	}

	rec = c.do("PUT", "/api/v1/pantry/"+a.ID, map[string]any{"productId": a.ProductID, "quantity": 100, "unit": "ml", "expiresOn": "2026-12-01"})
	var u pantryItemJSON
	decode(t, rec, &u)
	if rec.Code != 200 || *u.Quantity != 100 || u.ExpiresOn != "2026-12-01" {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}

	var list pantryListJSON
	decode(t, c.do("GET", "/api/v1/pantry", nil), &list)
	if len(list.Items) != 1 {
		t.Fatalf("list: %+v", list)
	}
	if rec := c.do("DELETE", "/api/v1/pantry/"+a.ID, nil); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("DELETE", "/api/v1/pantry/"+a.ID, nil); rec.Code != 404 {
		t.Fatalf("delete again: %d", rec.Code)
	}
}

func TestPantryValidationAndIsolation(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	for name, body := range map[string]map[string]any{
		"no product or name": {"quantity": 1, "unit": "g"},
		"negative":           {"name": "Sól", "quantity": -1, "unit": "g"},
		"bad date":           {"name": "Sól", "quantity": 1, "unit": "g", "expiresOn": "jutro"},
	} {
		if rec := c.do("POST", "/api/v1/pantry", body); rec.Code != 422 {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}

	var a pantryItemJSON
	decode(t, c.do("POST", "/api/v1/pantry", map[string]any{"name": "Sól", "quantity": 1000, "unit": "g"}), &a)
	if rec := c.do("PUT", "/api/v1/pantry/"+a.ID, map[string]any{"productId": a.ProductID, "quantity": 0, "unit": "g"}); rec.Code != 200 {
		t.Errorf("put zero: %d %s", rec.Code, rec.Body)
	}
	other := e.secondHouseholdClient(t)
	if rec := other.do("PUT", "/api/v1/pantry/"+a.ID, map[string]any{"name": "Sól", "quantity": 1, "unit": "g"}); rec.Code != 404 {
		t.Errorf("foreign put: %d", rec.Code)
	}
	if rec := other.do("DELETE", "/api/v1/pantry/"+a.ID, nil); rec.Code != 404 {
		t.Errorf("foreign delete: %d", rec.Code)
	}
	var list pantryListJSON
	decode(t, other.do("GET", "/api/v1/pantry", nil), &list)
	if len(list.Items) != 0 {
		t.Errorf("foreign list: %+v", list)
	}
	if rec := c.do("PUT", "/api/v1/pantry/nope", map[string]any{"name": "Sól", "quantity": 1, "unit": "g"}); rec.Code != 404 {
		t.Errorf("missing put: %d", rec.Code)
	}
}

func TestPantryReview(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	rec := c.do("POST", "/api/v1/pantry/review", nil)
	var r struct {
		ReviewedAt string `json:"reviewedAt"`
	}
	decode(t, rec, &r)
	if rec.Code != 200 || r.ReviewedAt != "2026-11-01T12:00:00Z" {
		t.Fatalf("review: %d %s", rec.Code, rec.Body)
	}
	var list pantryListJSON
	decode(t, c.do("GET", "/api/v1/pantry", nil), &list)
	if list.ReviewedAt == nil || *list.ReviewedAt != r.ReviewedAt {
		t.Fatalf("list reviewedAt: %+v", list)
	}
}

func TestMoveToPantryOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	rec := c.do("POST", "/api/v1/shopping/items", map[string]any{"week": "2026-11-02", "name": "Ryż", "quantity": 500, "unit": "g"})
	var it itemJSON
	decode(t, rec, &it)
	c.do("PATCH", "/api/v1/shopping/items/"+it.ID, map[string]any{"checked": true})

	var mv struct {
		Moved int `json:"moved"`
	}
	rec = c.do("POST", "/api/v1/shopping/move-to-pantry", map[string]any{"week": "2026-11-02"})
	decode(t, rec, &mv)
	if rec.Code != 200 || mv.Moved != 1 {
		t.Fatalf("move: %d %s", rec.Code, rec.Body)
	}
	rec = c.do("POST", "/api/v1/shopping/move-to-pantry", map[string]any{"week": "2026-11-02"})
	decode(t, rec, &mv)
	if rec.Code != 200 || mv.Moved != 0 {
		t.Fatalf("second move: %d %s", rec.Code, rec.Body)
	}
	var list pantryListJSON
	decode(t, c.do("GET", "/api/v1/pantry", nil), &list)
	if len(list.Items) != 1 || list.Items[0].Name == "" || *list.Items[0].Quantity != 500 {
		t.Fatalf("pantry: %+v", list)
	}
	var sl struct {
		Items []struct {
			Moved        bool     `json:"moved"`
			HaveQuantity *float64 `json:"haveQuantity"`
		} `json:"items"`
	}
	decode(t, c.do("GET", "/api/v1/shopping?week=2026-11-02", nil), &sl)
	if len(sl.Items) != 1 || !sl.Items[0].Moved {
		t.Fatalf("shopping: %+v", sl)
	}
	if rec := c.do("POST", "/api/v1/shopping/move-to-pantry", map[string]any{"week": "bad"}); rec.Code != 422 {
		t.Fatalf("bad week: %d", rec.Code)
	}
}

func TestPlanDoneAdjustsPantryOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Naleśniki") // 250 g mąki / 4 porcje
	var flour pantryItemJSON
	decode(t, c.do("POST", "/api/v1/pantry", map[string]any{"name": "Mąka", "quantity": 1000, "unit": "g"}), &flour)

	rec := c.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "dinner", "recipeId": r.ID})
	var en entryJSON
	decode(t, rec, &en)
	put := func(done bool) {
		rec := c.do("PUT", "/api/v1/plan/2026-11-02/entries/"+en.ID, map[string]any{"date": "2026-11-03", "slot": "dinner", "recipeId": r.ID, "servings": 4, "done": done})
		if rec.Code != 200 {
			t.Fatalf("put done=%v: %d %s", done, rec.Code, rec.Body)
		}
	}
	qty := func() float64 {
		var l pantryListJSON
		decode(t, c.do("GET", "/api/v1/pantry", nil), &l)
		for _, it := range l.Items {
			if it.ID == flour.ID {
				return *it.Quantity
			}
		}
		t.Fatalf("flour missing: %+v", l)
		return 0
	}
	put(true)
	if q := qty(); q != 750 {
		t.Fatalf("after done: %v", q)
	}
	put(false)
	if q := qty(); q != 1000 {
		t.Fatalf("after undone: %v", q)
	}
}
