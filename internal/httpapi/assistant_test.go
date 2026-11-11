package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"mealplan/internal/assistant"
)

// fakeAI answers every model call with reply, or with err when set.
type fakeAI struct {
	mu    sync.Mutex
	calls int
	err   error
	reply string
}

func (f *fakeAI) Complete(context.Context, string, string, string, string, json.RawMessage) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.reply, f.err
}

const testKey = "sk-or-v1-abcdef0123456789"

type assistantStatus struct {
	Configured   bool   `json:"configured"`
	KeyHint      string `json:"keyHint"`
	Model        string `json:"model"`
	DefaultModel string `json:"defaultModel"`
}

func TestAssistantSettingsAdminOnly(t *testing.T) {
	e := newEnv(t)
	admin := e.client(t)
	setup(t, admin)
	member, _ := inviteMember(t, e, admin, "mia")

	rec := admin.do("PUT", "/api/v1/household/assistant", map[string]any{"key": testKey, "model": "vendor/model-x"})
	if rec.Code != 200 {
		t.Fatalf("admin put: %d %s", rec.Code, rec.Body)
	}
	var st assistantStatus
	decode(t, rec, &st)
	if !st.Configured || st.KeyHint != testKey[len(testKey)-4:] || st.Model != "vendor/model-x" || st.DefaultModel != assistant.DefaultModel {
		t.Fatalf("%+v", st)
	}

	if rec := member.do("PUT", "/api/v1/household/assistant", map[string]any{"key": "sk-or-v1-other", "model": "vendor/m"}); rec.Code != 403 {
		t.Fatalf("member put: %d", rec.Code)
	}
	if rec := member.do("DELETE", "/api/v1/household/assistant", nil); rec.Code != 403 {
		t.Fatalf("member delete: %d", rec.Code)
	}
	for _, c := range []*client{admin, member} {
		rec := c.do("GET", "/api/v1/household/assistant", nil)
		if rec.Code != 200 {
			t.Fatalf("get: %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "sk-or") || strings.Contains(body, `"key"`) {
			t.Fatalf("the key leaked: %s", body)
		}
	}

	// A PUT without key keeps the key; DELETE removes it and keeps the model.
	if rec := admin.do("PUT", "/api/v1/household/assistant", map[string]any{"model": "vendor/model-y"}); rec.Code != 200 {
		t.Fatalf("put without key: %d %s", rec.Code, rec.Body)
	}
	decode(t, admin.do("GET", "/api/v1/household/assistant", nil), &st)
	if !st.Configured || st.Model != "vendor/model-y" {
		t.Fatalf("key must stay: %+v", st)
	}
	if rec := admin.do("DELETE", "/api/v1/household/assistant", nil); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	decode(t, admin.do("GET", "/api/v1/household/assistant", nil), &st)
	if st.Configured || st.KeyHint != "" || st.Model != "vendor/model-y" {
		t.Fatalf("after delete: %+v", st)
	}
}

func TestAssistantIngredientsMapsErrors(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	body := map[string]any{"ingredients": []map[string]any{{"name": "kwarkowy wynalazek", "quantity": 2, "unit": "szt"}}}

	if rec := c.do("POST", "/api/v1/assistant/ingredients", body); rec.Code != 409 || errCode(t, rec) != "assistant_not_configured" {
		t.Fatalf("no key: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/household/assistant/test", nil); rec.Code != 409 {
		t.Fatalf("test without key: %d", rec.Code)
	}
	if rec := c.do("PUT", "/api/v1/household/assistant", map[string]any{"key": testKey, "model": ""}); rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	for _, tc := range []struct {
		err  error
		code string
	}{
		{assistant.ErrBadKey, "assistant_bad_key"},
		{assistant.ErrNoCredit, "assistant_no_credit"},
		{fmt.Errorf("%w: status 400", assistant.ErrBadModel), "assistant_bad_model"},
		{fmt.Errorf("%w: model said no", assistant.ErrUnavailable), "assistant_unavailable"},
	} {
		e.ai.err = tc.err
		paths := []string{"/api/v1/assistant/ingredients", "/api/v1/household/assistant/test"}
		if tc.code == "assistant_bad_model" {
			paths = paths[:1] // the model-call limit is 10 a minute
		}
		for _, path := range paths {
			var b any = body
			if strings.HasSuffix(path, "/test") {
				b = nil
			}
			rec := c.do("POST", path, b)
			if rec.Code != 502 || errCode(t, rec) != tc.code {
				t.Fatalf("%s %v: %d %s", path, tc.err, rec.Code, rec.Body)
			}
			if tc.code == "assistant_unavailable" && !strings.Contains(rec.Body.String(), "model said no") {
				t.Fatalf("unavailable detail not surfaced: %s", rec.Body)
			}
		}
	}
	e.ai.err = errors.New("boom")
	if rec := c.do("POST", "/api/v1/assistant/ingredients", body); rec.Code != 500 {
		t.Fatalf("unknown error: %d", rec.Code)
	}
}

func TestAssistantIngredientsReturnsSuggestions(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	c.do("PUT", "/api/v1/household/assistant", map[string]any{"key": testKey, "model": ""})
	e.ai.reply = `{"suggestions":[{"index":0,"kind":"new","productId":null,"newProduct":{"name":"Kwarkowy wynalazek","kcal":30,"protein":3,"carbs":4,"fat":0.7,"pieceGrams":null,"densityGPerMl":null,"aliases":["kwarkowy wynalazki"]},"reason":"nowy"}]}`
	rec := c.do("POST", "/api/v1/assistant/ingredients", map[string]any{"ingredients": []map[string]any{{"name": "kwarkowy wynalazek", "quantity": nil, "unit": ""}}})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var out struct {
		Suggestions []struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			NewProduct *struct {
				Name    string   `json:"name"`
				Aliases []string `json:"aliases"`
			} `json:"newProduct"`
		} `json:"suggestions"`
	}
	decode(t, rec, &out)
	if len(out.Suggestions) != 1 || out.Suggestions[0].Kind != "new" || out.Suggestions[0].NewProduct == nil {
		t.Logf("%s", rec.Body)
		t.Fatalf("%+v", out)
	}
}

func TestAssistantReadEndpointsAndDuplicates(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)

	var cands struct {
		Candidates []any `json:"candidates"`
	}
	rec := c.do("GET", "/api/v1/assistant/ingredient-candidates", nil)
	decode(t, rec, &cands)
	if rec.Code != 200 || cands.Candidates == nil {
		t.Fatalf("candidates: %d %s", rec.Code, rec.Body)
	}
	var batches struct {
		Batches [][]string `json:"batches"`
	}
	rec = c.do("GET", "/api/v1/assistant/product-batches", nil)
	decode(t, rec, &batches)
	if rec.Code != 200 || batches.Batches == nil {
		t.Fatalf("batches: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/assistant/duplicates", map[string]any{"productIds": []string{"a", "b"}}); rec.Code != 409 {
		t.Fatalf("duplicates without key: %d", rec.Code)
	}
	c.do("PUT", "/api/v1/household/assistant", map[string]any{"key": testKey, "model": ""})
	rec = c.do("POST", "/api/v1/assistant/duplicates", map[string]any{"productIds": []string{"nope1", "nope2"}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"groups":[]`) {
		t.Fatalf("duplicates: %d %s", rec.Code, rec.Body)
	}
}

func TestModelCallsAreRateLimited(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	c.do("PUT", "/api/v1/household/assistant", map[string]any{"key": testKey, "model": ""})
	e.ai.err = assistant.ErrBadKey
	var last int
	for i := 0; i < 11; i++ {
		last = c.do("POST", "/api/v1/household/assistant/test", nil).Code
	}
	if last != 429 {
		t.Fatalf("11th call: %d", last)
	}
}

func createTestProduct(t *testing.T, c *client, name string, aliases ...string) productJSON {
	t.Helper()
	rec := c.do("POST", "/api/v1/products", productBody(name, aliases...))
	if rec.Code != 201 {
		t.Fatalf("create %s: %d %s", name, rec.Code, rec.Body)
	}
	var p productJSON
	decode(t, rec, &p)
	return p
}

func TestProductAssignEndpoint(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	p := createTestProduct(t, c, "Mąka testowa")
	var r recipeJSON
	body := recipeBody("Naleśniki")
	body["ingredients"] = []map[string]any{{"name": "Kwarkowy wynalazek", "quantity": 1, "unit": "szt"}}
	decode(t, c.do("POST", "/api/v1/recipes", body), &r)

	rec := c.do("POST", "/api/v1/products/"+p.ID+"/assign", map[string]any{"name": "Kwarkowy wynalazek", "recipeIds": []string{r.ID}})
	if rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Updated       int    `json:"updated"`
		AliasAdded    bool   `json:"aliasAdded"`
		AliasConflict string `json:"aliasConflict"`
	}
	decode(t, rec, &res)
	if res.Updated != 1 || !res.AliasAdded || res.AliasConflict != "" {
		t.Fatalf("%+v", res)
	}
	var usage struct{ Recipes, Pantry, FoodLog, Shopping int }
	decode(t, c.do("GET", "/api/v1/products/"+p.ID+"/usage", nil), &usage)
	if usage.Recipes != 1 {
		t.Fatalf("usage: %+v", usage)
	}

	if rec := c.do("POST", "/api/v1/products/missing/assign", map[string]any{"name": "Mąka", "recipeIds": []string{}}); rec.Code != 404 {
		t.Fatalf("unknown product: %d", rec.Code)
	}
	if rec := c.do("GET", "/api/v1/products/missing/usage", nil); rec.Code != 404 {
		t.Fatalf("usage unknown: %d", rec.Code)
	}
	other := e.secondHouseholdClient(t)
	if rec := other.do("POST", "/api/v1/products/"+p.ID+"/assign", map[string]any{"name": "Mąka", "recipeIds": []string{}}); rec.Code != 404 {
		t.Fatalf("foreign product: %d", rec.Code)
	}
	if rec := other.do("GET", "/api/v1/products/"+p.ID+"/usage", nil); rec.Code != 404 {
		t.Fatalf("foreign usage: %d", rec.Code)
	}
}

func TestProductMergeEndpoint(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	keep := createTestProduct(t, c, "Cebula testowa")
	dup := createTestProduct(t, c, "Cebula testowe")
	var r recipeJSON
	body := recipeBody("Zupa")
	body["ingredients"] = []map[string]any{{"name": "Kwarkowy wynalazek", "quantity": 1, "unit": "szt"}}
	decode(t, c.do("POST", "/api/v1/recipes", body), &r)
	c.do("POST", "/api/v1/products/"+dup.ID+"/assign", map[string]any{"name": "Kwarkowy wynalazek", "recipeIds": []string{r.ID}})

	rec := c.do("POST", "/api/v1/products/"+keep.ID+"/merge", map[string]any{"absorbIds": []string{dup.ID}})
	if rec.Code != 200 {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Repointed      struct{ Recipes, Pantry, FoodLog, Shopping int } `json:"repointed"`
		SkippedAliases []string                                         `json:"skippedAliases"`
	}
	decode(t, rec, &res)
	if res.Repointed.Recipes != 1 || res.SkippedAliases == nil {
		t.Fatalf("%+v (%s)", res, rec.Body)
	}
	// The absorbed product is gone, so a batch that overlaps by one id gets a clean 404.
	if rec := c.do("POST", "/api/v1/products/"+keep.ID+"/merge", map[string]any{"absorbIds": []string{dup.ID}}); rec.Code != 404 {
		t.Fatalf("already absorbed: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/products/"+keep.ID+"/merge", map[string]any{"absorbIds": []string{keep.ID}}); rec.Code != 422 {
		t.Fatalf("self merge: %d", rec.Code)
	}
	if rec := c.do("POST", "/api/v1/products/"+keep.ID+"/merge", map[string]any{"absorbIds": []string{}}); rec.Code != 422 {
		t.Fatalf("empty: %d", rec.Code)
	}
	other := e.secondHouseholdClient(t)
	foreign := createTestProduct(t, other, "Obca")
	if rec := c.do("POST", "/api/v1/products/"+keep.ID+"/merge", map[string]any{"absorbIds": []string{foreign.ID}}); rec.Code != 404 {
		t.Fatalf("foreign absorb: %d %s", rec.Code, rec.Body)
	}
	if rec := other.do("POST", "/api/v1/products/"+keep.ID+"/merge", map[string]any{"absorbIds": []string{foreign.ID}}); rec.Code != 404 {
		t.Fatalf("foreign keep: %d %s", rec.Code, rec.Body)
	}
}
