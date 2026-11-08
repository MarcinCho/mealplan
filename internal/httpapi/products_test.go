package httpapi_test

import (
	"context"
	"testing"

	"mealplan/internal/domain"
)

type productJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Per100g struct {
		Kcal, Protein, Carbs, Fat *float64
	} `json:"per100g"`
	PieceGrams    *float64 `json:"pieceGrams"`
	DensityGPerML *float64 `json:"densityGPerMl"`
	Aliases       []string `json:"aliases"`
}

func productBody(name string, aliases ...string) map[string]any {
	return map[string]any{
		"name": name, "per100g": map[string]any{"kcal": 60, "protein": 3, "carbs": 5, "fat": nil},
		"pieceGrams": nil, "densityGPerMl": 1.03, "aliases": aliases,
	}
}

func TestProductCRUDOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	rec := c.do("POST", "/api/v1/products", productBody("Mleko testowe", "Mleczko Testowe"))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var p productJSON
	decode(t, rec, &p)
	if p.ID == "" || p.Per100g.Fat != nil || p.Per100g.Kcal == nil || *p.DensityGPerML != 1.03 || len(p.Aliases) != 1 || p.Aliases[0] != "mleczko testowe" {
		t.Fatalf("created: %+v", p)
	}

	var list []productJSON
	decode(t, c.do("GET", "/api/v1/products?q=MLECZKO+test", nil), &list)
	if len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("q list: %+v", list)
	}
	var all []productJSON
	decode(t, c.do("GET", "/api/v1/products", nil), &all)
	if len(all) < 2 {
		t.Fatalf("seed expected: %d", len(all))
	}

	rec = c.do("PUT", "/api/v1/products/"+p.ID, productBody("Mleko zmienione"))
	if rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	decode(t, rec, &p)
	if p.Name != "Mleko zmienione" || p.Aliases == nil || len(p.Aliases) != 0 {
		t.Fatalf("updated: %+v", p)
	}
	if rec := c.do("DELETE", "/api/v1/products/"+p.ID, nil); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestProductValidationConflictAndAuth(t *testing.T) {
	e := newEnv(t)
	if rec := e.client(t).do("GET", "/api/v1/products", nil); rec.Code != 401 {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	c := e.client(t)
	setup(t, c)
	if rec := c.do("POST", "/api/v1/products", productBody("")); rec.Code != 422 || errCode(t, rec) != "invalid_input" {
		t.Fatalf("empty name: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/products", `{"name":"x","bogus":1}`); rec.Code != 400 {
		t.Fatalf("unknown field: %d", rec.Code)
	}
	c.do("POST", "/api/v1/products", productBody("Unikat"))
	if rec := c.do("POST", "/api/v1/products", productBody("unikat")); rec.Code != 409 || errCode(t, rec) != "conflict" {
		t.Fatalf("duplicate: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/products", productBody("Inny", "UNIKAT")); rec.Code != 409 || errCode(t, rec) != "conflict" {
		t.Fatalf("alias=name: %d %s", rec.Code, rec.Body)
	}
}

func TestProductInUseAndIsolation(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	var p productJSON
	decode(t, a.do("POST", "/api/v1/products", productBody("Jadalny")), &p)

	var me struct{ User struct{ ID string } }
	decode(t, a.do("GET", "/api/v1/auth/me", nil), &me)
	var hid string
	if err := e.st.DB().QueryRow(e.st.DB().Rebind(`SELECT household_id FROM product WHERE id = ?`), p.ID).Scan(&hid); err != nil {
		t.Fatal(err)
	}
	err := e.st.AddFoodLog(context.Background(), domain.FoodLogEntry{ID: "l1", HouseholdID: hid, UserID: me.User.ID, Date: "2026-11-01", ProductID: p.ID, Quantity: 1, Unit: "g", CreatedAt: e.clock.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if rec := a.do("DELETE", "/api/v1/products/"+p.ID, nil); rec.Code != 409 || errCode(t, rec) != "in_use" {
		t.Fatalf("in use: %d %s", rec.Code, rec.Body)
	}

	b := e.secondHouseholdClient(t)
	if rec := b.do("PUT", "/api/v1/products/"+p.ID, productBody("Hacked")); rec.Code != 404 {
		t.Fatalf("update foreign: %d", rec.Code)
	}
	if rec := b.do("DELETE", "/api/v1/products/"+p.ID, nil); rec.Code != 404 {
		t.Fatalf("delete foreign: %d", rec.Code)
	}
	var list []productJSON
	decode(t, b.do("GET", "/api/v1/products?q=jadalny", nil), &list)
	if len(list) != 0 {
		t.Fatalf("leak: %+v", list)
	}
}
