package pantry

import (
	"context"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/units"
)

// Deductions lists the stock decreases for cooking the recipe for the given servings. Only ingredients
// with a quantity that match a product held in stock in the same base unit (and not as a staple) count.
func Deductions(r domain.Recipe, servings int, m *nutrition.Matcher, stock []domain.PantryItem, entryID string) []domain.PantryMovement {
	if r.BaseServings <= 0 || servings <= 0 {
		return nil
	}
	have := map[[2]string]domain.PantryItem{}
	for _, it := range stock {
		have[[2]string{it.ProductID, it.Unit}] = it
	}
	totals := map[[2]string]float64{}
	var order [][2]string
	for _, ing := range r.Ingredients {
		if ing.Quantity == nil || *ing.Quantity <= 0 {
			continue
		}
		p, ok := m.Match(ing)
		if !ok {
			continue
		}
		base, unit := units.ToBase(*ing.Quantity, ing.Unit)
		if unit == "" {
			unit = "szt" // a unit-less amount counts pieces, as on the shopping list
		}
		key := [2]string{p.ID, unit}
		it, ok := have[key]
		if !ok || it.Staple || it.Quantity == nil {
			continue
		}
		amount, err := units.Scale(base, servings, r.BaseServings)
		if err != nil || amount <= 0 {
			continue
		}
		if _, seen := totals[key]; !seen {
			order = append(order, key)
		}
		totals[key] += amount
	}
	var out []domain.PantryMovement
	for _, k := range order {
		out = append(out, domain.PantryMovement{EntryID: entryID, ProductID: k[0], Unit: k[1], Amount: -units.Round(totals[k])})
	}
	return out
}

// EntryDone takes the recipe's ingredients out of stock. It does nothing for free-text entries; the
// store applies the deductions atomically and only if the entry has no recorded movements yet.
func (s *Service) EntryDone(ctx context.Context, householdID string, e domain.PlanEntry) error {
	if e.RecipeID == "" {
		return nil
	}
	r, err := s.recipes.RecipeByID(ctx, householdID, e.RecipeID)
	if err != nil {
		return err
	}
	stock, err := s.repo.PantryItems(ctx, householdID)
	if err != nil {
		return err
	}
	all, err := s.products.All(ctx, householdID)
	if err != nil {
		return err
	}
	ms := Deductions(r, e.Servings, nutrition.NewMatcher(all), stock, e.ID)
	if len(ms) == 0 {
		return nil
	}
	_, err = s.repo.ConsumeForEntry(ctx, householdID, e.ID, ms)
	return err
}

// EntryUndone puts back what EntryDone took (the amounts actually applied) and forgets the movements,
// atomically; a repeated call finds nothing to do.
func (s *Service) EntryUndone(ctx context.Context, householdID, entryID string) error {
	return s.repo.RestoreForEntry(ctx, householdID, entryID)
}
