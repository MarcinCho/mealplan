package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/store"
)

var (
	admin  = auth.Principal{HouseholdID: "h1", Role: domain.RoleAdmin}
	member = auth.Principal{HouseholdID: "h1", Role: domain.RoleMember}
)

var serviceProducts = fakeProducts{
	{ID: "p-onion", Name: "cebula", NormalizedName: "cebula", Aliases: []string{"cebulka"}},
	{ID: "p-red", Name: "cebula czerwona", NormalizedName: "cebula czerwona"},
	{ID: "p-tomato", Name: "pomidor", NormalizedName: "pomidor"},
	{ID: "p-tomatoes", Name: "pomidory", NormalizedName: "pomidory"},
}

func configured(t *testing.T, c Client) (*Service, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	repo.settings["h1"] = [2]string{testKey, ""}
	return NewService(repo, serviceProducts, c), repo
}

func TestMatchIngredientsExactSkipsModel(t *testing.T) {
	fc := &fakeClient{reply: func(string) (string, error) {
		return `{"suggestions":[{"index":0,"kind":"existing","productId":"p-red","newProduct":null,"reason":"szyk"}]}`, nil
	}}
	s, _ := configured(t, fc)
	got, err := s.MatchIngredients(context.Background(), member, []IngredientQuery{{Name: "Cebulka", Quantity: fp(1), Unit: "szt"}, {Name: "czerwona cebula", Quantity: fp(2), Unit: "szt"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Suggestion{
		{Name: "Cebulka", Kind: KindExisting, ProductID: "p-onion", ProductName: "cebula", Reason: "exact"},
		{Name: "czerwona cebula", Kind: KindExisting, ProductID: "p-red", ProductName: "cebula czerwona", Reason: "szyk"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if fc.calls != 1 || fc.key != testKey || fc.model != DefaultModel {
		t.Fatalf("calls %d key ok %v model %q", fc.calls, fc.key == testKey, fc.model)
	}
	var sent struct {
		Ingredients []struct {
			Index int
			Name  string
		}
		Products []struct{ ID string }
	}
	if err := json.Unmarshal([]byte(fc.users[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Ingredients) != 1 || sent.Ingredients[0].Name != "czerwona cebula" || sent.Ingredients[0].Index != 0 || len(sent.Products) != 4 {
		t.Fatalf("sent %s", fc.users[0])
	}
}

func TestMatchIngredientsAllExact(t *testing.T) {
	fc := &fakeClient{}
	s, _ := configured(t, fc)
	got, err := s.MatchIngredients(context.Background(), member, []IngredientQuery{{Name: "pomidor"}})
	if err != nil || len(got) != 1 || got[0].ProductID != "p-tomato" || fc.calls != 0 {
		t.Fatalf("got %+v err %v calls %d", got, err, fc.calls)
	}
}

func TestMatchIngredientsLimits(t *testing.T) {
	fc := &fakeClient{}
	s, _ := configured(t, fc)
	qs := make([]IngredientQuery, 26)
	for i := range qs {
		qs[i].Name = "x"
	}
	if _, err := s.MatchIngredients(context.Background(), member, qs); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("26 queries: err = %v", err)
	}
	for _, q := range []IngredientQuery{{Name: ""}, {Name: strings.Repeat("a", 201)}, {Name: "a\x00b"}, {Name: "a", Unit: strings.Repeat("u", 51)}} {
		if _, err := s.MatchIngredients(context.Background(), member, []IngredientQuery{q}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("%+v: err = %v", q, err)
		}
	}
	if fc.calls != 0 {
		t.Fatalf("calls = %d", fc.calls)
	}
}

func TestMatchIngredientsUsesStoredModel(t *testing.T) {
	fc := &fakeClient{reply: func(string) (string, error) { return `{"suggestions":[]}`, nil }}
	s, repo := configured(t, fc)
	repo.settings["h1"] = [2]string{testKey, "vendor/other"}
	got, err := s.MatchIngredients(context.Background(), member, []IngredientQuery{{Name: "kmin"}})
	if err != nil || fc.model != "vendor/other" || got[0].Kind != KindNone {
		t.Fatalf("got %+v err %v model %q", got, err, fc.model)
	}
}

func TestMatchIngredientsClientError(t *testing.T) {
	fc := &fakeClient{reply: func(string) (string, error) { return "", ErrNoCredit }}
	s, _ := configured(t, fc)
	if _, err := s.MatchIngredients(context.Background(), member, []IngredientQuery{{Name: "kmin"}}); !errors.Is(err, ErrNoCredit) {
		t.Fatalf("err = %v", err)
	}
}

func TestNotConfigured(t *testing.T) {
	fc := &fakeClient{}
	s := NewService(newFakeRepo(), serviceProducts, fc)
	ctx := context.Background()
	if _, err := s.MatchIngredients(ctx, member, []IngredientQuery{{Name: "kmin"}}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("match: err = %v", err)
	}
	if _, err := s.FindDuplicates(ctx, member, []string{"p-tomato", "p-tomatoes"}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("duplicates: err = %v", err)
	}
	if err := s.Test(ctx, member); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("test: err = %v", err)
	}
	if fc.calls != 0 {
		t.Fatalf("calls = %d", fc.calls)
	}
}

func TestConfigureAdminOnly(t *testing.T) {
	repo := newFakeRepo()
	s := NewService(repo, serviceProducts, &fakeClient{})
	ctx := context.Background()
	key := testKey
	if err := s.Configure(ctx, member, &key, ""); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("member: err = %v", err)
	}
	if _, ok := repo.settings["h1"]; ok {
		t.Fatal("member changed the settings")
	}
	if err := s.Configure(ctx, admin, &key, "vendor/model-1:free"); err != nil {
		t.Fatal(err)
	}
	if repo.settings["h1"] != [2]string{testKey, "vendor/model-1:free"} {
		t.Fatalf("settings = %v", repo.settings["h1"])
	}
	// nil key keeps the key; an empty model means the default.
	if err := s.Configure(ctx, admin, nil, ""); err != nil {
		t.Fatal(err)
	}
	if k, m, _ := repo.AssistantSettings(ctx, "h1"); k != testKey || m != "" {
		t.Fatalf("after nil key: %q %q", k, m)
	}
	if _, _, model, _ := s.Status(ctx, member); model != "" {
		t.Fatalf("model = %q, want none stored", model)
	}
	empty := ""
	if err := s.Configure(ctx, admin, &empty, "vendor/m"); err != nil {
		t.Fatal(err)
	}
	if k, _, _ := repo.AssistantSettings(ctx, "h1"); k != "" {
		t.Fatal("empty key did not remove the key")
	}
}

func TestConfigureValidation(t *testing.T) {
	repo := newFakeRepo()
	s := NewService(repo, serviceProducts, &fakeClient{})
	ctx := context.Background()
	bad := []struct{ key, model string }{
		{"sk-or with space", ""},
		{"sk-or-\tx", ""},
		{"sk-or-\x00x", ""},
		{strings.Repeat("k", 201), ""},
		{"k", "no-slash"},
		{"k", "Vendor/Model"},
		{"k", "vendor/model name"},
		{"k", "vendor/" + strings.Repeat("m", 94)},
		{"k", "https://evil/x"},
	}
	for _, b := range bad {
		key := b.key
		err := s.Configure(ctx, admin, &key, b.model)
		if !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("%q %q: err = %v", b.key, b.model, err)
		}
		if b.key != "k" && strings.Contains(err.Error(), b.key) {
			t.Fatalf("error repeats the key: %v", err)
		}
	}
	if len(repo.settings) != 0 {
		t.Fatalf("invalid input was stored: %v", repo.settings)
	}
	ok := strings.Repeat("k", 200)
	if err := s.Configure(ctx, admin, &ok, "vendor/"+strings.Repeat("m", 93)); err != nil {
		t.Fatalf("limits: %v", err)
	}
}

func TestStatusHidesKey(t *testing.T) {
	repo := newFakeRepo()
	s := NewService(repo, serviceProducts, &fakeClient{})
	ctx := context.Background()
	if ok, hint, model, err := s.Status(ctx, member); err != nil || ok || hint != "" || model != "" {
		t.Fatalf("empty: %v %q %q %v", ok, hint, model, err)
	}
	repo.settings["h1"] = [2]string{"sk-or-v1-0123456789abcd", "vendor/m"}
	ok, hint, model, err := s.Status(ctx, member)
	if err != nil || !ok || hint != "abcd" || model != "vendor/m" {
		t.Fatalf("got %v %q %q %v", ok, hint, model, err)
	}
}

const noneReply = `{"suggestions":[{"index":0,"kind":"none","productId":"","newProduct":{"name":"","kcal":null,"protein":null,"carbs":null,"fat":null,"pieceGrams":null,"densityGPerMl":null,"aliases":[]},"reason":"test"}]}`

func TestServiceTest(t *testing.T) {
	fc := &fakeClient{reply: func(string) (string, error) { return noneReply, nil }}
	s, _ := configured(t, fc)
	if err := s.Test(context.Background(), member); err != nil || fc.calls != 1 {
		t.Fatalf("err %v calls %d", err, fc.calls)
	}
	// Test goes through the production matching path: same instructions, same schema.
	if fc.systems[0] != matchSystem || string(fc.schemas[0]) != string(matchSchema) || !strings.Contains(fc.users[0], `"sól"`) {
		t.Fatalf("system match %v schema match %v user %s", fc.systems[0] == matchSystem, string(fc.schemas[0]) == string(matchSchema), fc.users[0])
	}
	fc.reply = func(string) (string, error) { return `{"ok":true}`, nil }
	if err := s.Test(context.Background(), member); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("answer outside the schema: err = %v", err)
	}
	fc.reply = func(string) (string, error) { return "", ErrBadKey }
	if err := s.Test(context.Background(), member); !errors.Is(err, ErrBadKey) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceTestThroughOpenRouter(t *testing.T) {
	var schema json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ResponseFormat struct {
				JSONSchema struct {
					Schema json.RawMessage `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		schema = body.ResponseFormat.JSONSchema.Schema
		content, _ := json.Marshal(noneReply)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":`+string(content)+`}}]}`)
	}))
	defer srv.Close()
	s, _ := configured(t, newTestClient(srv.URL, 5*time.Second))
	if err := s.Test(context.Background(), member); err != nil {
		t.Fatal(err)
	}
	var sent, want any
	if json.Unmarshal(schema, &sent) != nil || json.Unmarshal(matchSchema, &want) != nil || !reflect.DeepEqual(sent, want) {
		t.Fatalf("Test sent schema %s", schema)
	}
}

func TestFindDuplicates(t *testing.T) {
	fc := &fakeClient{reply: func(string) (string, error) {
		return `{"groups":[{"keepId":"p-tomato","absorbIds":["p-tomatoes","p-onion"],"reason":"liczba mnoga"}]}`, nil
	}}
	s, _ := configured(t, fc)
	ctx := context.Background()
	got, err := s.FindDuplicates(ctx, member, []string{"p-tomato", "p-tomatoes", "p-unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []DuplicateGroup{{KeepID: "p-tomato", AbsorbIDs: []string{"p-tomatoes"}, Reason: "liczba mnoga"}}) {
		t.Fatalf("got %+v", got)
	}
	var sent struct{ Products []struct{ ID, Name string } }
	if err := json.Unmarshal([]byte(fc.users[0]), &sent); err != nil || len(sent.Products) != 2 {
		t.Fatalf("sent %s (%v)", fc.users[0], err)
	}
	ids := make([]string, 81)
	for i := range ids {
		ids[i] = "p-tomato"
	}
	if _, err := s.FindDuplicates(ctx, member, ids); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("81 ids: err = %v", err)
	}
	if got, err := s.FindDuplicates(ctx, member, []string{"p-onion"}); err != nil || len(got) != 0 || fc.calls != 1 {
		t.Fatalf("one product: %+v %v calls %d", got, err, fc.calls)
	}
}

func TestIngredientCandidatesAndBatches(t *testing.T) {
	s, repo := configured(t, &fakeClient{})
	repo.refs["h1"] = []store.IngredientRef{{RecipeID: "r1", RecipeTitle: "Zupa", Name: "kmin", NormalizedName: "kmin"}}
	ctx := context.Background()
	cs, err := s.IngredientCandidates(ctx, member)
	if err != nil || len(cs) != 1 || cs[0].NormalizedName != "kmin" {
		t.Fatalf("candidates %+v %v", cs, err)
	}
	bs, err := s.ProductBatches(ctx, member)
	if err != nil || !reflect.DeepEqual(bs, [][]string{{"p-onion", "p-red", "p-tomato", "p-tomatoes"}}) {
		t.Fatalf("batches %v %v", bs, err)
	}
}
