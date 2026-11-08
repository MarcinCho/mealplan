package nutrition

import (
	"mealplan/internal/domain"
	"mealplan/internal/units"
)

// Matcher finds the product for an ingredient.
type Matcher struct {
	byID   map[string]domain.Product
	byName map[string]domain.Product
}

// NewMatcher indexes products by ID, normalized name and aliases. On name
// clashes the first product wins.
func NewMatcher(products []domain.Product) *Matcher {
	m := &Matcher{byID: map[string]domain.Product{}, byName: map[string]domain.Product{}}
	add := func(key string, p domain.Product) {
		if key == "" {
			return
		}
		if _, ok := m.byName[key]; !ok {
			m.byName[key] = p
		}
	}
	for _, p := range products {
		m.byID[p.ID] = p
	}
	for _, p := range products {
		add(units.NormalizeName(p.Name), p)
		add(p.NormalizedName, p)
	}
	for _, p := range products {
		for _, a := range p.Aliases {
			add(units.NormalizeName(a), p)
		}
	}
	return m
}

// Match prefers the remembered ProductID, then falls back to name/alias.
func (m *Matcher) Match(ing domain.Ingredient) (domain.Product, bool) {
	if ing.ProductID != "" {
		if p, ok := m.byID[ing.ProductID]; ok {
			return p, true
		}
	}
	key := units.NormalizeName(ing.Name)
	if key == "" {
		key = ing.NormalizedName
	}
	p, ok := m.byName[key]
	return p, ok
}
