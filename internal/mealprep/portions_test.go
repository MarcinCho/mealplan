package mealprep_test

import (
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/mealprep"
)

func TestPortionAccounting(t *testing.T) {
	d := domain.PrepDish{ID: "d1", Portions: 12, WrittenOff: 1}
	entries := []domain.PlanEntry{
		{ID: "e1", Kind: domain.EntryPrep, PrepDishID: "d1", Servings: 2},
		{ID: "e2", Kind: domain.EntryPrep, PrepDishID: "d1", Servings: 2},
		{ID: "e3", Kind: domain.EntryPrep, PrepDishID: "other", Servings: 5},
	}
	items := []domain.FreezerItem{{ID: "f1", DishID: "d1", Portions: 4}}
	if a, f, r := mealprep.DishAllocated("d1", entries), mealprep.DishFrozen("d1", items), mealprep.DishRemaining(d, items, entries); a != 4 || f != 4 || r != 3 {
		t.Fatalf("allocated %d frozen %d remaining %d", a, f, r)
	}
	if r := mealprep.DishRemaining(d, items, entries[1:]); r != 5 {
		t.Fatalf("after delete remaining %d", r)
	}
}
