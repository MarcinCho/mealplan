package assistant

import (
	"errors"
	"reflect"
	"testing"

	"mealplan/internal/domain"
)

var parseProducts = []domain.Product{
	{ID: "p-onion", Name: "cebula", NormalizedName: "cebula", Aliases: []string{"cebulka"}},
	{ID: "p-tomato", Name: "pomidor", NormalizedName: "pomidor", Aliases: []string{"pomidory"}},
}

func TestParseRejectsForeignIDs(t *testing.T) {
	qs := []IngredientQuery{{Name: "cebula szalotka"}, {Name: "pomidorki"}, {Name: "sól"}}
	raw := `{"suggestions":[
		{"index":0,"kind":"existing","productId":"p-invented","newProduct":null,"reason":"podobne"},
		{"index":1,"kind":"existing","productId":"p-tomato","newProduct":null,"reason":"liczba mnoga"},
		{"index":2,"kind":"existing","productId":null,"newProduct":null,"reason":"?"},
		{"index":7,"kind":"existing","productId":"p-onion","newProduct":null,"reason":"poza listą"}
	]}`
	got, err := parseMatch(raw, qs, parseProducts)
	if err != nil {
		t.Fatal(err)
	}
	want := []Suggestion{
		{Name: "cebula szalotka", Kind: KindNone},
		{Name: "pomidorki", Kind: KindExisting, ProductID: "p-tomato", ProductName: "pomidor", Reason: "liczba mnoga"},
		{Name: "sól", Kind: KindNone},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseRejectsInvalidJSON(t *testing.T) {
	for _, raw := range []string{``, `not json`, `{"suggestions":"x"}`, `[1,2]`} {
		if _, err := parseMatch(raw, []IngredientQuery{{Name: "x"}}, parseProducts); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%q: err = %v", raw, err)
		}
	}
	if _, err := parseDuplicates(`{"groups":7}`, []string{"a", "b"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("duplicates: err = %v", err)
	}
}

func TestParseUnknownKindIsNone(t *testing.T) {
	got, err := parseMatch(`{"suggestions":[{"index":0,"kind":"maybe","productId":"p-onion","newProduct":null,"reason":"x"}]}`,
		[]IngredientQuery{{Name: "cebulki"}}, parseProducts)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Kind != KindNone || got[0].ProductID != "" {
		t.Fatalf("got %+v", got[0])
	}
}

func TestParseNewProductOutOfRange(t *testing.T) {
	for name, np := range map[string]string{
		"kcal 1200":   `{"name":"masło klarowane","kcal":1200,"protein":0,"carbs":0,"fat":99,"pieceGrams":null,"densityGPerMl":null,"aliases":[]}`,
		"negative":    `{"name":"masło klarowane","kcal":800,"protein":-1,"carbs":0,"fat":99,"pieceGrams":null,"densityGPerMl":null,"aliases":[]}`,
		"macro 101":   `{"name":"masło klarowane","kcal":800,"protein":0,"carbs":0,"fat":101,"pieceGrams":null,"densityGPerMl":null,"aliases":[]}`,
		"piece 6000":  `{"name":"masło klarowane","kcal":800,"protein":0,"carbs":0,"fat":99,"pieceGrams":6000,"densityGPerMl":null,"aliases":[]}`,
		"empty name":  `{"name":"   ","kcal":800,"protein":0,"carbs":0,"fat":99,"pieceGrams":null,"densityGPerMl":null,"aliases":[]}`,
		"null object": `null`,
	} {
		raw := `{"suggestions":[{"index":0,"kind":"new","productId":null,"newProduct":` + np + `,"reason":"brak w bazie"}]}`
		got, err := parseMatch(raw, []IngredientQuery{{Name: "ghee"}}, parseProducts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got[0].Kind != KindNone || got[0].New != nil {
			t.Fatalf("%s: got %+v", name, got[0])
		}
	}
}

func TestParseNewProductValid(t *testing.T) {
	raw := `{"suggestions":[{"index":0,"kind":"new","productId":null,"newProduct":
		{"name":" Cebula  Czerwona ","kcal":40,"protein":1.1,"carbs":9.3,"fat":0.1,"pieceGrams":150,"densityGPerMl":null,
		 "aliases":["czerwona cebula","Czerwona  cebula","cebulka","",  "cebula czerwona"]},"reason":"odmiana"}]}`
	got, err := parseMatch(raw, []IngredientQuery{{Name: "czerwona cebula"}}, parseProducts)
	if err != nil {
		t.Fatal(err)
	}
	s := got[0]
	if s.Kind != KindNew || s.New == nil || s.ProductID != "" {
		t.Fatalf("got %+v", s)
	}
	// "cebulka" already belongs to cebula, so it is not proposed again; the name itself is not an alias.
	if s.New.Name != "Cebula Czerwona" {
		t.Fatalf("name %q", s.New.Name)
	}
	if !reflect.DeepEqual(s.New.Aliases, []string{"czerwona cebula"}) {
		t.Fatalf("aliases %q", s.New.Aliases)
	}
	if *s.New.Per100g.Kcal != 40 || *s.New.PieceGrams != 150 || s.New.DensityGPerML != nil || s.Reason != "odmiana" {
		t.Fatalf("got %+v", s.New)
	}
}

func TestParseNewProductThatExists(t *testing.T) {
	for _, name := range []string{"Pomidory", "pomidor"} {
		raw := `{"suggestions":[{"index":0,"kind":"new","productId":null,"newProduct":
			{"name":"` + name + `","kcal":18,"protein":0.9,"carbs":3.9,"fat":0.2,"pieceGrams":null,"densityGPerMl":null,"aliases":[]},"reason":"nowy"}]}`
		got, err := parseMatch(raw, []IngredientQuery{{Name: "pomidorów"}}, parseProducts)
		if err != nil {
			t.Fatal(err)
		}
		want := Suggestion{Name: "pomidorów", Kind: KindExisting, ProductID: "p-tomato", ProductName: "pomidor", Reason: "nowy"}
		if !reflect.DeepEqual(got[0], want) {
			t.Fatalf("%s: got %+v", name, got[0])
		}
	}
}

func TestParseDuplicates(t *testing.T) {
	batch := []string{"a", "b", "c", "d", "e"}
	raw := `{"groups":[
		{"keepId":"a","absorbIds":["b","zz","a","b"],"reason":"liczba mnoga"},
		{"keepId":"zz","absorbIds":["c"],"reason":"obce keep"},
		{"keepId":"c","absorbIds":[],"reason":"pusta"},
		{"keepId":"c","absorbIds":["b","d"],"reason":"b już było"},
		{"keepId":"b","absorbIds":["e"],"reason":"b już było"},
		{"keepId":"e","absorbIds":["a"],"reason":"a już było"}
	]}`
	got, err := parseDuplicates(raw, batch)
	if err != nil {
		t.Fatal(err)
	}
	want := []DuplicateGroup{
		{KeepID: "a", AbsorbIDs: []string{"b"}, Reason: "liczba mnoga"},
		{KeepID: "c", AbsorbIDs: []string{"d"}, Reason: "b już było"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
