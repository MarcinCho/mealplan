package nutrition

import (
	"testing"

	"mealplan/internal/domain"
)

func TestMatchPrefersRememberedProduct(t *testing.T) {
	a := domain.Product{ID: "a", Name: "ser"}
	b := domain.Product{ID: "b", Name: "ser żółty"}
	m := NewMatcher([]domain.Product{a, b})
	p, ok := m.Match(domain.Ingredient{Name: "ser", ProductID: "b"})
	if !ok || p.ID != "b" {
		t.Fatalf("got %v %v", p.ID, ok)
	}
}

func TestMatchByNormalizedNameAndAlias(t *testing.T) {
	m := NewMatcher([]domain.Product{
		{ID: "1", Name: "mąka pszenna"},
		{ID: "2", Name: "mąka żytnia", Aliases: []string{"mąki"}},
	})
	if p, ok := m.Match(domain.Ingredient{Name: "  Mąka   pszenna "}); !ok || p.ID != "1" {
		t.Errorf("name: %v %v", p.ID, ok)
	}
	if p, ok := m.Match(domain.Ingredient{Name: "mąki"}); !ok || p.ID != "2" {
		t.Errorf("alias: %v %v", p.ID, ok)
	}
}

func TestMatchUnknownProductID(t *testing.T) {
	m := NewMatcher([]domain.Product{{ID: "1", Name: "mleko"}})
	p, ok := m.Match(domain.Ingredient{Name: "mleko", ProductID: "gone"})
	if !ok || p.ID != "1" {
		t.Fatalf("got %v %v", p.ID, ok)
	}
}

func TestNoMatch(t *testing.T) {
	m := NewMatcher([]domain.Product{{ID: "1", Name: "mleko"}})
	if _, ok := m.Match(domain.Ingredient{Name: "ryż"}); ok {
		t.Fatal("unexpected match")
	}
}
