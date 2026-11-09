package mealprep

import "mealplan/internal/domain"

// DishAllocated is the number of portions of the dish planned as prep entries.
func DishAllocated(dishID string, entries []domain.PlanEntry) int {
	n := 0
	for _, e := range entries {
		if e.Kind == domain.EntryPrep && e.PrepDishID == dishID {
			n += e.Servings
		}
	}
	return n
}

// DishFrozen is the number of portions of the dish moved to the freezer.
func DishFrozen(dishID string, items []domain.FreezerItem) int {
	n := 0
	for _, f := range items {
		if f.DishID == dishID {
			n += f.Portions
		}
	}
	return n
}

// DishRemaining is what is left of the dish: Portions - allocated - frozen - written off.
func DishRemaining(d domain.PrepDish, items []domain.FreezerItem, entries []domain.PlanEntry) int {
	return d.Portions - DishAllocated(d.ID, entries) - DishFrozen(d.ID, items) - d.WrittenOff
}

// FreezerAllocated is the number of portions of the freezer item planned as prep entries.
func FreezerAllocated(itemID string, entries []domain.PlanEntry) int {
	n := 0
	for _, e := range entries {
		if e.Kind == domain.EntryPrep && e.FreezerItemID == itemID {
			n += e.Servings
		}
	}
	return n
}

// FreezerRemaining is the freezer item's portions not yet planned.
func FreezerRemaining(f domain.FreezerItem, entries []domain.PlanEntry) int {
	return f.Portions - FreezerAllocated(f.ID, entries)
}
