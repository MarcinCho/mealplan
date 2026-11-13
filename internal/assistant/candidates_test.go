package assistant

import (
	"reflect"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func TestCandidateIngredients(t *testing.T) {
	products := []domain.Product{
		{ID: "p-onion", Name: "cebula", NormalizedName: "cebula"},
		{ID: "p-shallot", Name: "szalotka", NormalizedName: "szalotka", Aliases: []string{"cebulka"}},
	}
	refs := []store.IngredientRef{
		{RecipeID: "r1", RecipeTitle: "Gulasz", Name: "Czerwona cebula", NormalizedName: "czerwona cebula"},
		{RecipeID: "r1", RecipeTitle: "Gulasz", Name: "cebula", NormalizedName: "cebula"},
		{RecipeID: "r2", RecipeTitle: "Sałatka", Name: "czerwona cebula", NormalizedName: "czerwona cebula"},
		{RecipeID: "r2", RecipeTitle: "Sałatka", Name: "czerwona  cebula", NormalizedName: "czerwona cebula"}, // same recipe twice
		{RecipeID: "r3", RecipeTitle: "Tortilla", Name: "czerwona cebula", NormalizedName: "czerwona cebula", ProductID: "p-onion"},
		{RecipeID: "r3", RecipeTitle: "Tortilla", Name: "cebulka", NormalizedName: "cebulka", ProductID: "p-shallot"},
		{RecipeID: "r4", RecipeTitle: "Zupa", Name: "Cebula", NormalizedName: "cebula", ProductID: "p-onion"},
		{RecipeID: "r4", RecipeTitle: "Zupa", Name: "  ", NormalizedName: ""},
		{RecipeID: "r4", RecipeTitle: "Zupa", Name: "anyż", NormalizedName: "anyż"},
	}
	got := CandidateIngredients(refs, products)
	want := []IngredientCandidate{
		{Name: "anyż", NormalizedName: "anyż", RecipeIDs: []string{"r4"}, RecipeTitles: []string{"Zupa"}},
		{Name: "Czerwona cebula", NormalizedName: "czerwona cebula", RecipeIDs: []string{"r1", "r2"}, RecipeTitles: []string{"Gulasz", "Sałatka"}},
		{Name: "czerwona cebula", NormalizedName: "czerwona cebula", CurrentProductID: "p-onion", RecipeIDs: []string{"r3"}, RecipeTitles: []string{"Tortilla"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestCandidateIngredientsEmpty(t *testing.T) {
	if got := CandidateIngredients(nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil slice", got)
	}
}

func TestCandidateProductBatches(t *testing.T) {
	products := []domain.Product{
		{ID: "t2", Name: "pomidory"},
		{ID: "e", Name: "jajko"},
		{ID: "t3", Name: "Pomidorki koktajlowe"},
		{ID: "t1", Name: "pomidor"},
		{ID: "s2", Name: "sery"},
		{ID: "s1", Name: "ser"},
	}
	got := CandidateProductBatches(products, 80)
	want := [][]string{{"t1", "t3", "t2", "s1", "s2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCandidateProductBatchesPacking(t *testing.T) {
	products := []domain.Product{
		{ID: "a1", Name: "mąka pszenna"}, {ID: "a2", Name: "mąka żytnia"},
		{ID: "b1", Name: "ser żółty"}, {ID: "b2", Name: "ser biały"}, {ID: "b3", Name: "serek wiejski"},
		{ID: "c1", Name: "jabłko"}, {ID: "c2", Name: "jabłka"},
		{ID: "x", Name: "sól"},
		{ID: "d1", Name: "ryż"}, {ID: "d2", Name: "ryż basmati"}, {ID: "d3", Name: "ryż brązowy"}, {ID: "d4", Name: "ryż jaśminowy"}, {ID: "d5", Name: "ryżu"},
	}
	// Groups (sorted): jab(2) mąk(2) ryż(5, with "ryżu") ser(3, with "serek"); sól has no neighbour.
	// ryż is larger than max: chunk d1-d4, then the lone d5 is paired with d4.
	got := CandidateProductBatches(products, 4)
	want := [][]string{{"c2", "c1", "a1", "a2"}, {"d1", "d2", "d3", "d4"}, {"d4", "d5"}, {"b2", "b1", "b3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// A group larger than max is split into consecutive batches and never loses an id.
	big := []domain.Product{{ID: "1", Name: "ryż a"}, {ID: "2", Name: "ryż b"}, {ID: "3", Name: "ryż c"}, {ID: "4", Name: "ryż d"}, {ID: "5", Name: "ryż e"}}
	if got := CandidateProductBatches(big, 2); !reflect.DeepEqual(got, [][]string{{"1", "2"}, {"3", "4"}, {"4", "5"}}) {
		t.Fatalf("split with lone remainder: got %v", got)
	}
	if got := CandidateProductBatches(big[:4], 3); !reflect.DeepEqual(got, [][]string{{"1", "2", "3"}, {"3", "4"}}) {
		t.Fatalf("split 4 by 3: got %v", got)
	}
	// A remainder of two or more is a batch of its own and other groups can join it.
	six := append(append([]domain.Product{}, big...), domain.Product{ID: "6", Name: "ryż f"}, domain.Product{ID: "x1", Name: "sól"}, domain.Product{ID: "x2", Name: "sól morska"})
	if got := CandidateProductBatches(six, 4); !reflect.DeepEqual(got, [][]string{{"1", "2", "3", "4"}, {"5", "6", "x1", "x2"}}) {
		t.Fatalf("split with pair remainder: got %v", got)
	}
}
