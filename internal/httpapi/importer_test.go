package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/importer"
)

type fakeImporter struct {
	draft importer.Draft
	err   error
	calls []importer.Source
}

func (f *fakeImporter) Import(_ context.Context, src importer.Source) (importer.Draft, error) {
	f.calls = append(f.calls, src)
	return f.draft, f.err
}

func qty(v float64) *float64 { return &v }

type draftResp struct {
	Title       string   `json:"title"`
	Servings    int      `json:"servings"`
	SourceURL   string   `json:"sourceUrl"`
	Tags        []string `json:"tags"`
	Ingredients []struct {
		Raw        string   `json:"raw"`
		Name       string   `json:"name"`
		Quantity   *float64 `json:"quantity"`
		Unit       string   `json:"unit"`
		Note       string   `json:"note"`
		Confidence string   `json:"confidence"`
	} `json:"ingredients"`
	Steps    []string `json:"steps"`
	Warnings []string `json:"warnings"`
}

func sampleDraft() importer.Draft {
	return importer.Draft{
		Title: "Zupa", Servings: 4, SourceURL: "https://example.com/z", Tags: []string{"zupa"},
		Ingredients: []importer.IngredientLine{
			{Raw: "200 g mąki", Name: "mąki", Quantity: qty(200), Unit: "g", Confidence: "ok"},
			{Raw: "sól do smaku", Name: "sól", Note: "do smaku", Confidence: "ok"},
		},
		Steps:    []string{"Gotuj."},
		Warnings: []string{"uwaga"},
	}
}

func TestImportURLReturnsDraftAndSavesNothing(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = sampleDraft()

	rec := c.do("POST", "/api/v1/recipes/import/url", map[string]any{"url": "https://example.com/z"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var d draftResp
	decode(t, rec, &d)
	if d.Title != "Zupa" || d.Servings != 4 || len(d.Ingredients) != 2 || d.Ingredients[0].Quantity == nil || *d.Ingredients[0].Quantity != 200 ||
		d.Ingredients[1].Quantity != nil || d.Ingredients[1].Note != "do smaku" || d.Warnings[0] != "uwaga" {
		t.Fatalf("%+v", d)
	}
	if len(e.imp.calls) != 1 || e.imp.calls[0].Kind != importer.KindURL || e.imp.calls[0].URL != "https://example.com/z" {
		t.Fatalf("calls=%+v", e.imp.calls)
	}
	var list []recipeJSON
	decode(t, c.do("GET", "/api/v1/recipes", nil), &list)
	if len(list) != 0 {
		t.Fatalf("import must not save: %+v", list)
	}
}

func TestImportTextReturnsDraft(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = sampleDraft()
	rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "Zupa\n1 l wody"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(e.imp.calls) != 1 || e.imp.calls[0].Kind != importer.KindText || e.imp.calls[0].Text != "Zupa\n1 l wody" {
		t.Fatalf("calls=%+v", e.imp.calls)
	}
}

func TestImportEmptySlicesAreArraysNotNull(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = importer.Draft{Title: "x"}
	rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"})
	body := rec.Body.String()
	for _, want := range []string{`"tags":[]`, `"ingredients":[]`, `"steps":[]`, `"warnings":[]`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
}

func TestImportErrorMapping(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{importer.ErrBlocked, 422, "import_blocked"},
		{importer.ErrUnparsable, 422, "import_unparsable"},
		{importer.ErrFetch, 422, "import_failed"},
		{importer.ErrTimeout, 504, "import_timeout"},
		{domain.ErrInvalid, 422, "invalid_input"},
	}
	for _, tc := range cases {
		e.imp.err = tc.err
		rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"})
		if rec.Code != tc.status || errCode(t, rec) != tc.code {
			t.Errorf("%v: %d %s", tc.err, rec.Code, rec.Body)
		}
	}
}

func TestImportRequiresSessionCSRFAndKnownFields(t *testing.T) {
	e := newEnv(t)
	anon := e.client(t)
	if rec := anon.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"}); rec.Code != 401 {
		t.Errorf("anonymous: %d", rec.Code)
	}
	c := e.client(t)
	setup(t, c)
	if rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x", "extra": 1}); rec.Code != 400 {
		t.Errorf("unknown field: %d", rec.Code)
	}
}

func TestImportURLIsRateLimitedPerUser(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = sampleDraft()
	for i := 0; i < 10; i++ {
		if rec := c.do("POST", "/api/v1/recipes/import/url", map[string]any{"url": "https://example.com/z"}); rec.Code != 200 {
			t.Fatalf("call %d: %d", i, rec.Code)
		}
	}
	rec := c.do("POST", "/api/v1/recipes/import/url", map[string]any{"url": "https://example.com/z"})
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("11th call: %d %v", rec.Code, rec.Header())
	}
	if rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"}); rec.Code != 200 {
		t.Errorf("text import must not be limited: %d", rec.Code)
	}
}
