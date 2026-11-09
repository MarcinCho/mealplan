package httpapi_test

import (
	"strings"
	"testing"
)

type allocJSON struct {
	EntryID  string `json:"entryId"`
	Date     string `json:"date"`
	Slot     string `json:"slot"`
	Servings int    `json:"servings"`
}

type dishOut struct {
	ID          string      `json:"id"`
	RecipeID    string      `json:"recipeId"`
	RecipeTitle string      `json:"recipeTitle"`
	Portions    int         `json:"portions"`
	WrittenOff  int         `json:"writtenOff"`
	Allocated   int         `json:"allocated"`
	Frozen      int         `json:"frozen"`
	Remaining   int         `json:"remaining"`
	Allocations []allocJSON `json:"allocations"`
}

type sessionOut struct {
	ID        string    `json:"id"`
	Date      string    `json:"date"`
	Name      string    `json:"name"`
	Week      string    `json:"week"`
	Cooked    bool      `json:"cooked"`
	Remaining int       `json:"remaining"`
	Dishes    []dishOut `json:"dishes"`
}

type freezerOut struct {
	ID          string      `json:"id"`
	DishID      *string     `json:"dishId"`
	RecipeID    *string     `json:"recipeId"`
	Title       string      `json:"title"`
	Portions    int         `json:"portions"`
	Allocated   int         `json:"allocated"`
	Remaining   int         `json:"remaining"`
	FrozenOn    string      `json:"frozenOn"`
	Allocations []allocJSON `json:"allocations"`
}

func getSessions(t *testing.T, c *client) []sessionOut {
	t.Helper()
	var l struct {
		Sessions []sessionOut `json:"sessions"`
	}
	decode(t, c.do("GET", "/api/v1/prep/sessions", nil), &l)
	return l.Sessions
}

func TestPrepSessionFlow(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Gulasz")

	if got := getSessions(t, c); got == nil || len(got) != 0 {
		t.Fatalf("empty list: %#v", got)
	}
	rec := c.do("POST", "/api/v1/prep/sessions", map[string]any{"date": "2026-11-08", "name": "Niedziela"})
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var s sessionOut
	decode(t, rec, &s)
	if s.Week != "2026-11-09" || s.Name != "Niedziela" || s.Cooked || s.Dishes == nil {
		t.Fatalf("session: %+v", s)
	}

	rec = c.do("POST", "/api/v1/prep/sessions/"+s.ID+"/dishes", map[string]any{"recipeId": r.ID, "portions": 12})
	if rec.Code != 201 {
		t.Fatalf("dish: %d %s", rec.Code, rec.Body)
	}
	var d dishOut
	decode(t, rec, &d)
	if d.RecipeTitle != "Gulasz" || d.Portions != 12 || d.Remaining != 12 || d.Allocations == nil {
		t.Fatalf("dish: %+v", d)
	}

	rec = c.do("POST", "/api/v1/prep/dishes/"+d.ID+"/freeze", map[string]any{"portions": 4, "frozenOn": "2026-11-09"})
	if rec.Code != 201 {
		t.Fatalf("freeze: %d %s", rec.Code, rec.Body)
	}
	var fz freezerOut
	decode(t, rec, &fz)
	if fz.DishID == nil || *fz.DishID != d.ID || fz.Portions != 4 || fz.Title != "Gulasz" || fz.Remaining != 4 || fz.FrozenOn != "2026-11-09" {
		t.Fatalf("freezer: %+v", fz)
	}

	var ids []string
	for _, day := range []string{"2026-11-10", "2026-11-11"} {
		rec = c.do("POST", "/api/v1/plan/2026-11-09/entries", map[string]any{"date": day, "slot": "dinner", "kind": "prep", "prepDishId": d.ID, "servings": 1})
		if rec.Code != 201 {
			t.Fatalf("prep entry: %d %s", rec.Code, rec.Body)
		}
		var en struct {
			ID         string    `json:"id"`
			Kind       string    `json:"kind"`
			PrepDishID *string   `json:"prepDishId"`
			FreezerID  *string   `json:"freezerItemId"`
			OutValues  *struct{} `json:"outValues"`
		}
		decode(t, rec, &en)
		if en.Kind != "prep" || en.PrepDishID == nil || *en.PrepDishID != d.ID || en.FreezerID != nil || en.OutValues != nil {
			t.Fatalf("entry: %s", rec.Body)
		}
		ids = append(ids, en.ID)
	}

	check := func(allocated, remaining, n int) {
		t.Helper()
		got := getSessions(t, c)
		if len(got) != 1 || len(got[0].Dishes) != 1 {
			t.Fatalf("sessions: %+v", got)
		}
		g := got[0].Dishes[0]
		if g.Allocated != allocated || g.Frozen != 4 || g.Remaining != remaining || len(g.Allocations) != n || got[0].Remaining != remaining {
			t.Fatalf("dish: %+v (session remaining %d)", g, got[0].Remaining)
		}
	}
	check(2, 6, 2)
	if a := getSessions(t, c)[0].Dishes[0].Allocations[0]; a.EntryID == "" || a.Date != "2026-11-10" || a.Slot != "dinner" || a.Servings != 1 {
		t.Fatalf("allocation: %+v", a)
	}

	if rec = c.do("DELETE", "/api/v1/plan/2026-11-09/entries/"+ids[0], nil); rec.Code != 204 {
		t.Fatalf("delete entry: %d", rec.Code)
	}
	if rec = c.do("DELETE", "/api/v1/plan/2026-11-09/entries/"+ids[1], nil); rec.Code != 204 {
		t.Fatalf("delete entry: %d", rec.Code)
	}
	check(0, 8, 0)

	rec = c.do("PUT", "/api/v1/prep/dishes/"+d.ID, map[string]any{"portions": 12, "writtenOff": 1})
	var u dishOut
	decode(t, rec, &u)
	if rec.Code != 200 || u.WrittenOff != 1 || u.Remaining != 7 {
		t.Fatalf("update dish: %d %s", rec.Code, rec.Body)
	}
	rec = c.do("PUT", "/api/v1/prep/sessions/"+s.ID, map[string]any{"date": "2026-11-08", "name": "Nowa", "week": "2026-11-09", "cooked": false})
	var us sessionOut
	decode(t, rec, &us)
	if rec.Code != 200 || us.Name != "Nowa" {
		t.Fatalf("update session: %d %s", rec.Code, rec.Body)
	}

	// Freezer CRUD.
	var fl struct {
		Items []freezerOut `json:"items"`
	}
	decode(t, c.do("GET", "/api/v1/freezer", nil), &fl)
	if len(fl.Items) != 1 || fl.Items[0].ID != fz.ID {
		t.Fatalf("freezer list: %+v", fl)
	}
	rec = c.do("POST", "/api/v1/freezer", map[string]any{"label": "Pierogi", "portions": 6, "frozenOn": "2026-11-01"})
	var man freezerOut
	decode(t, rec, &man)
	if rec.Code != 201 || man.DishID != nil || man.RecipeID != nil || man.Title != "Pierogi" || man.FrozenOn != "2026-11-01" {
		t.Fatalf("manual freezer: %d %s", rec.Code, rec.Body)
	}
	rec = c.do("PUT", "/api/v1/freezer/"+man.ID, map[string]any{"label": "Pierogi ruskie", "portions": 8, "frozenOn": "2026-11-01"})
	var mu freezerOut
	decode(t, rec, &mu)
	if rec.Code != 200 || mu.Portions != 8 || mu.Title != "Pierogi ruskie" {
		t.Fatalf("update freezer: %d %s", rec.Code, rec.Body)
	}
	if rec = c.do("DELETE", "/api/v1/freezer/"+man.ID, nil); rec.Code != 204 {
		t.Fatalf("delete freezer: %d %s", rec.Code, rec.Body)
	}
	if rec = c.do("DELETE", "/api/v1/prep/dishes/"+d.ID, nil); rec.Code != 204 {
		t.Fatalf("delete dish: %d %s", rec.Code, rec.Body)
	}
	if rec = c.do("DELETE", "/api/v1/prep/sessions/"+s.ID, nil); rec.Code != 204 {
		t.Fatalf("delete session: %d %s", rec.Code, rec.Body)
	}
}

func TestPrepDeleteInUse409(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	r := createRecipe(t, c, "Gulasz")
	var s sessionOut
	decode(t, c.do("POST", "/api/v1/prep/sessions", map[string]any{"date": "2026-11-08", "name": "N"}), &s)
	var d dishOut
	decode(t, c.do("POST", "/api/v1/prep/sessions/"+s.ID+"/dishes", map[string]any{"recipeId": r.ID, "portions": 4}), &d)
	var fz freezerOut
	decode(t, c.do("POST", "/api/v1/prep/dishes/"+d.ID+"/freeze", map[string]any{"portions": 2}), &fz)
	for _, body := range []map[string]any{
		{"date": "2026-11-10", "slot": "dinner", "kind": "prep", "prepDishId": d.ID, "servings": 1},
		{"date": "2026-11-11", "slot": "dinner", "kind": "prep", "freezerItemId": fz.ID, "servings": 1},
	} {
		if rec := c.do("POST", "/api/v1/plan/2026-11-09/entries", body); rec.Code != 201 {
			t.Fatalf("entry: %d %s", rec.Code, rec.Body)
		}
	}
	for _, path := range []string{"/api/v1/prep/sessions/" + s.ID, "/api/v1/prep/dishes/" + d.ID, "/api/v1/freezer/" + fz.ID} {
		rec := c.do("DELETE", path, nil)
		if rec.Code != 409 || !contains(rec.Body.String(), `"in_use"`) {
			t.Errorf("DELETE %s: %d %s", path, rec.Code, rec.Body)
		}
	}
}

func TestPrepForeignHousehold404(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	r := createRecipe(t, a, "Gulasz")
	var s sessionOut
	decode(t, a.do("POST", "/api/v1/prep/sessions", map[string]any{"date": "2026-11-08", "name": "N"}), &s)
	var d dishOut
	decode(t, a.do("POST", "/api/v1/prep/sessions/"+s.ID+"/dishes", map[string]any{"recipeId": r.ID, "portions": 4}), &d)
	var fz freezerOut
	decode(t, a.do("POST", "/api/v1/prep/dishes/"+d.ID+"/freeze", map[string]any{"portions": 2}), &fz)

	b := e.secondHouseholdClient(t)
	cases := []struct {
		method, path string
		body         any
	}{
		{"PUT", "/api/v1/prep/sessions/" + s.ID, map[string]any{"date": "2026-11-08", "name": "X", "week": "2026-11-09", "cooked": false}},
		{"DELETE", "/api/v1/prep/sessions/" + s.ID, nil},
		{"POST", "/api/v1/prep/sessions/" + s.ID + "/dishes", map[string]any{"recipeId": r.ID, "portions": 1}},
		{"PUT", "/api/v1/prep/dishes/" + d.ID, map[string]any{"portions": 4, "writtenOff": 0}},
		{"DELETE", "/api/v1/prep/dishes/" + d.ID, nil},
		{"POST", "/api/v1/prep/dishes/" + d.ID + "/freeze", map[string]any{"portions": 1}},
		{"PUT", "/api/v1/freezer/" + fz.ID, map[string]any{"label": "x", "portions": 1}},
		{"DELETE", "/api/v1/freezer/" + fz.ID, nil},
	}
	for _, tc := range cases {
		if rec := b.do(tc.method, tc.path, tc.body); rec.Code != 404 {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, rec.Code, rec.Body)
		}
	}
	if got := getSessions(t, b); len(got) != 0 {
		t.Fatalf("leak: %+v", got)
	}
}

func TestHouseholdSettings(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	var s struct {
		PortionsPerMeal int `json:"portionsPerMeal"`
	}
	decode(t, c.do("GET", "/api/v1/household/settings", nil), &s)
	if s.PortionsPerMeal != 2 {
		t.Fatalf("default: %+v", s)
	}
	rec := c.do("PUT", "/api/v1/household/settings", map[string]any{"portionsPerMeal": 3})
	decode(t, rec, &s)
	if rec.Code != 200 || s.PortionsPerMeal != 3 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	decode(t, c.do("GET", "/api/v1/household/settings", nil), &s)
	if s.PortionsPerMeal != 3 {
		t.Fatalf("persisted: %+v", s)
	}
	for _, n := range []int{0, 21} {
		if rec := c.do("PUT", "/api/v1/household/settings", map[string]any{"portionsPerMeal": n}); rec.Code != 422 {
			t.Errorf("%d: %d %s", n, rec.Code, rec.Body)
		}
	}
}

func TestOutEntryJSON(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	type out struct {
		ID        string `json:"id"`
		Kind      string `json:"kind"`
		OutValues *struct {
			Kcal    *float64 `json:"kcal"`
			Protein *float64 `json:"protein"`
			Carbs   *float64 `json:"carbs"`
			Fat     *float64 `json:"fat"`
		} `json:"outValues"`
	}
	rec := c.do("POST", "/api/v1/plan/2026-11-09/entries", map[string]any{"date": "2026-11-10", "slot": "lunch", "kind": "out", "freeText": "Pizzeria",
		"outValues": map[string]any{"kcal": 900, "protein": nil, "carbs": nil, "fat": nil}})
	var o out
	decode(t, rec, &o)
	if rec.Code != 201 || o.Kind != "out" || o.OutValues == nil || o.OutValues.Kcal == nil || *o.OutValues.Kcal != 900 || o.OutValues.Protein != nil {
		t.Fatalf("out: %d %s", rec.Code, rec.Body)
	}

	rec = c.do("POST", "/api/v1/plan/2026-11-09/entries", map[string]any{"date": "2026-11-10", "slot": "dinner", "freeText": "Kanapka"})
	var m out
	decode(t, rec, &m)
	if m.Kind != "meal" || m.OutValues != nil {
		t.Fatalf("meal: %s", rec.Body)
	}
	var wk struct{ Entries []out }
	decode(t, c.do("GET", "/api/v1/plan/2026-11-09", nil), &wk)
	kinds := map[string]string{}
	for _, en := range wk.Entries {
		kinds[en.ID] = en.Kind
	}
	if kinds[o.ID] != "out" || kinds[m.ID] != "meal" {
		t.Fatalf("week kinds: %+v", kinds)
	}

	// Nutrition days carry the kind.
	if rec := c.do("PUT", "/api/v1/me/nutrition", map[string]any{"enabled": true, "goal": map[string]any{"kcal": 2000, "protein": nil, "carbs": nil, "fat": nil}}); rec.Code != 200 {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	var days struct {
		Days []struct {
			Meals []struct {
				EntryID string
				Kind    string
			}
		}
	}
	decode(t, c.do("GET", "/api/v1/me/nutrition/days?from=2026-11-10&to=2026-11-10", nil), &days)
	if len(days.Days) != 1 {
		t.Fatalf("days: %+v", days)
	}
	got := map[string]string{}
	for _, mm := range days.Days[0].Meals {
		got[mm.EntryID] = mm.Kind
	}
	if got[o.ID] != "out" || got[m.ID] != "meal" {
		t.Fatalf("meal kinds: %+v", got)
	}
}

func TestCookSessionPantryFailedMapsTo500(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	var s sessionOut
	decode(t, c.do("POST", "/api/v1/prep/sessions", map[string]any{"date": "2026-11-08", "name": "N"}), &s)
	e.cookFail.Store(true)
	rec := c.do("PUT", "/api/v1/prep/sessions/"+s.ID, map[string]any{"date": "2026-11-08", "name": "N", "week": s.Week, "cooked": true})
	if rec.Code != 500 || !contains(rec.Body.String(), `"pantry_failed"`) {
		t.Fatalf("cook: %d %s", rec.Code, rec.Body)
	}
	e.cookFail.Store(false)
	rec = c.do("PUT", "/api/v1/prep/sessions/"+s.ID, map[string]any{"date": "2026-11-08", "name": "N", "week": s.Week, "cooked": true})
	var u sessionOut
	decode(t, rec, &u)
	if rec.Code != 200 || !u.Cooked {
		t.Fatalf("retry: %d %s", rec.Code, rec.Body)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
