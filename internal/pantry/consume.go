package pantry

import (
	"context"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/units"
)

// Deductions lists the stock decreases for cooking the recipe for the given servings. Only ingredients
// with a quantity that match a product held in stock (and not as a staple) count. The amount is taken
// in the stock's unit: the same base unit, or one nutrition.Convert reaches for that product (3 łyżki
// from a bottle in ml, 2 szklanki of flour from a bag in g when the product has a density).
func Deductions(r domain.Recipe, servings int, m *nutrition.Matcher, stock []domain.PantryItem, entryID string) []domain.PantryMovement {
	if r.BaseServings <= 0 || servings <= 0 {
		return nil
	}
	byProduct := map[string][]domain.PantryItem{}
	for _, it := range stock {
		byProduct[it.ProductID] = append(byProduct[it.ProductID], it)
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
		scaled, err := units.Scale(*ing.Quantity, servings, r.BaseServings)
		if err != nil || scaled <= 0 {
			continue
		}
		it, amount, ok := stockFor(byProduct[p.ID], scaled, ing.Unit, p)
		if !ok || it.Staple || it.Quantity == nil || amount <= 0 {
			continue
		}
		key := [2]string{p.ID, it.Unit}
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

// stockFor picks the product's stock item for an amount in unit, preferring the same base unit, and
// returns the amount in that item's unit.
func stockFor(items []domain.PantryItem, amount float64, unit string, p domain.Product) (domain.PantryItem, float64, bool) {
	base, u := units.ToBase(amount, unit)
	if u == "" {
		u = "szt" // a unit-less amount counts pieces, as on the shopping list
	}
	for _, it := range items {
		if it.Unit == u {
			return it, base, true
		}
	}
	for _, it := range items {
		if v, ok := nutrition.Convert(amount, unit, it.Unit, p); ok {
			return it, v, true
		}
	}
	return domain.PantryItem{}, 0, false
}

// EntryDone takes the recipe's ingredients out of stock. It does nothing for free-text, prep and out
// entries (the zero Kind is a meal, as in the store); the
// store applies the deductions atomically and only if the entry has no recorded movements yet.
func (s *Service) EntryDone(ctx context.Context, householdID string, e domain.PlanEntry) error {
	if (e.Kind != "" && e.Kind != domain.EntryMeal) || e.RecipeID == "" {
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

// SessionCooked takes the ingredients of every dish in the session out of stock, scaled to the
// dish's portions. Like EntryDone it applies once: movements already recorded make it a no-op.
func (s *Service) SessionCooked(ctx context.Context, householdID string, sess domain.PrepSession, dishes []domain.PrepDish) error {
	if len(dishes) == 0 {
		return nil
	}
	stock, err := s.repo.PantryItems(ctx, householdID)
	if err != nil {
		return err
	}
	all, err := s.products.All(ctx, householdID)
	if err != nil {
		return err
	}
	m := nutrition.NewMatcher(all)
	totals := map[[2]string]float64{}
	var order [][2]string
	for _, d := range dishes {
		r, err := s.recipes.RecipeByID(ctx, householdID, d.RecipeID)
		if err != nil {
			return err
		}
		for _, mv := range Deductions(r, d.Portions, m, stock, "") {
			k := [2]string{mv.ProductID, mv.Unit}
			if _, ok := totals[k]; !ok {
				order = append(order, k)
			}
			totals[k] += mv.Amount
		}
	}
	var ms []domain.PantryMovement
	for _, k := range order {
		ms = append(ms, domain.PantryMovement{SessionID: sess.ID, ProductID: k[0], Unit: k[1], Amount: units.Round(totals[k])})
	}
	if len(ms) == 0 {
		return nil
	}
	_, err = s.repo.ConsumeForSession(ctx, householdID, sess.ID, ms)
	return err
}

// SessionUncooked puts back what SessionCooked took; a repeated call finds nothing to do.
func (s *Service) SessionUncooked(ctx context.Context, householdID, sessionID string) error {
	return s.repo.RestoreForSession(ctx, householdID, sessionID)
}
