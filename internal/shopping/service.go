package shopping

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/pantry"
	"mealplan/internal/plan"
	"mealplan/internal/units"
)

const (
	maxName     = 200
	maxUnitLen  = 30
	maxQuantity = 1e6
)

type Repo interface {
	ShoppingItems(ctx context.Context, householdID, week string) ([]domain.ShoppingItem, error)
	ReplaceGeneratedItems(ctx context.Context, householdID, week string, items []domain.ShoppingItem) error
	InsertShoppingItem(ctx context.Context, it domain.ShoppingItem) error
	UpdateShoppingItem(ctx context.Context, it domain.ShoppingItem) error
	DeleteShoppingItem(ctx context.Context, householdID, id string) error
	ShoppingItemByID(ctx context.Context, householdID, id string) (domain.ShoppingItem, error)
	ClaimShoppingItemsForMove(ctx context.Context, householdID, week string) ([]domain.ShoppingItem, error)
	UnclaimShoppingItems(ctx context.Context, householdID string, ids []string) error
}

// Pantry is the part of pantry.Service the list needs.
type Pantry interface {
	List(ctx context.Context, householdID string) ([]domain.PantryItem, *time.Time, error)
	Add(ctx context.Context, householdID string, in pantry.Input) (domain.PantryItem, error)
}

// Products is the part of product.Service the list needs.
type Products interface {
	All(ctx context.Context, householdID string) ([]domain.Product, error)
}

type PlanSource interface {
	PlanEntries(ctx context.Context, householdID, fromDate, toDate string) ([]domain.PlanEntry, error)
	RecipesByIDs(ctx context.Context, householdID string, ids []string) (map[string]domain.Recipe, error)
}

// Sessions is the part of the meal prep store the list needs: sessions are shopped for by the week
// they belong to, whatever day range is generated.
type Sessions interface {
	PrepSessionsForWeek(ctx context.Context, householdID, week string) ([]domain.PrepSession, error)
	PrepDishes(ctx context.Context, householdID string, sessionIDs []string) ([]domain.PrepDish, error)
}

type ItemInput struct {
	Name     string
	Quantity *float64
	Unit     string
}

type Service struct {
	repo     Repo
	plans    PlanSource
	pantry   Pantry
	products Products
	sessions Sessions
}

// SetSessions makes the list include the dishes of the week's prep sessions.
func (s *Service) SetSessions(x Sessions) { s.sessions = x }

func New(repo Repo, plans PlanSource, pan Pantry, products Products) *Service {
	return &Service{repo: repo, plans: plans, pantry: pan, products: products}
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

func (in ItemInput) validate() (name, unit string, err error) {
	if domain.HasControl(in.Name) || domain.HasControl(in.Unit) {
		return "", "", invalid("name and unit must not contain control characters")
	}
	name = strings.TrimSpace(in.Name)
	unit = units.CanonUnit(in.Unit)
	if n := utf8.RuneCountInString(name); n == 0 || n > maxName {
		return "", "", invalid("name must be 1 to %d characters", maxName)
	}
	if utf8.RuneCountInString(unit) > maxUnitLen {
		return "", "", invalid("unit must be at most %d characters", maxUnitLen)
	}
	if in.Quantity != nil {
		v := *in.Quantity
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > maxQuantity {
			return "", "", invalid("quantity must be greater than 0 and at most %v", maxQuantity)
		}
	}
	return name, unit, nil
}

func itemKey(norm, unit string, hasQty bool) string {
	if !hasQty {
		return norm + "|-"
	}
	return norm + "|" + unit
}

func sameQty(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return units.Round(*a) == units.Round(*b)
}

func (s *Service) List(ctx context.Context, householdID, week string) ([]domain.ShoppingItem, error) {
	if _, err := plan.ValidWeek(week); err != nil {
		return nil, err
	}
	return s.repo.ShoppingItems(ctx, householdID, week)
}

// PreviousWeekUndone counts the recipe entries of the week before the given one that were not marked
// done: their ingredients may still be needed from stock the new list subtracts.
func (s *Service) PreviousWeekUndone(ctx context.Context, householdID, week string) (int, error) {
	start, err := plan.ValidWeek(week)
	if err != nil {
		return 0, err
	}
	const layout = "2006-01-02"
	entries, err := s.plans.PlanEntries(ctx, householdID, start.AddDate(0, 0, -7).Format(layout), start.AddDate(0, 0, -1).Format(layout))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if (e.Kind == "" || e.Kind == domain.EntryMeal) && e.RecipeID != "" && !e.Done {
			n++
		}
	}
	return n, nil
}

// Generate rebuilds the generated part of the week's list from the plan. Manual items are kept, and
// a generated item whose key and quantity did not change keeps its id and checked state.
func (s *Service) Generate(ctx context.Context, householdID, week, from, to string) ([]domain.ShoppingItem, error) {
	dates, err := plan.WeekDates(week)
	if err != nil {
		return nil, err
	}
	lo, hi := dates[0], dates[6]
	if from != "" || to != "" {
		if from == "" || to == "" || from > to || from < lo || to > hi {
			return nil, invalid("from and to must be dates of the week, from not after to")
		}
		if _, err := plan.ValidWeek(week); err != nil {
			return nil, err
		}
		lo, hi = from, to
	}
	entries, err := s.plans.PlanEntries(ctx, householdID, lo, hi)
	if err != nil {
		return nil, err
	}
	if s.sessions != nil {
		sessions, err := s.sessions.PrepSessionsForWeek(ctx, householdID, week)
		if err != nil {
			return nil, err
		}
		sids := make([]string, 0, len(sessions))
		cooked := map[string]bool{}
		for _, ps := range sessions {
			sids = append(sids, ps.ID)
			cooked[ps.ID] = ps.Cooked
		}
		if len(sids) > 0 {
			dishes, err := s.sessions.PrepDishes(ctx, householdID, sids)
			if err != nil {
				return nil, err
			}
			for _, d := range dishes {
				entries = append(entries, domain.PlanEntry{Kind: domain.EntryMeal, RecipeID: d.RecipeID, Servings: d.Portions, Done: cooked[d.SessionID]})
			}
		}
	}
	ids := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		if e.RecipeID != "" && !seen[e.RecipeID] {
			seen[e.RecipeID] = true
			ids = append(ids, e.RecipeID)
		}
	}
	recipes, err := s.plans.RecipesByIDs(ctx, householdID, ids)
	if err != nil {
		return nil, err
	}
	prods, err := s.products.All(ctx, householdID)
	if err != nil {
		return nil, err
	}
	matcher := nutrition.NewMatcher(prods)
	productOf := func(ing domain.Ingredient) string {
		if p, ok := matcher.Match(ing); ok {
			return p.ID
		}
		return ""
	}
	pantryItems, _, err := s.pantry.List(ctx, householdID)
	if err != nil {
		return nil, err
	}
	stock := make([]StockLine, 0, len(pantryItems))
	for _, p := range pantryItems {
		stock = append(stock, StockLine{ProductID: p.ProductID, Quantity: p.Quantity, Unit: p.Unit, Staple: p.Staple})
	}
	lines, err := Build(entries, recipes, productOf, stock)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.ShoppingItems(ctx, householdID, week)
	if err != nil {
		return nil, err
	}
	prev := map[string]domain.ShoppingItem{}
	for _, it := range existing {
		if !it.Manual {
			prev[itemKey(it.NormalizedName, it.Unit, it.Quantity != nil)] = it
		}
	}
	fresh := make([]domain.ShoppingItem, 0, len(lines))
	for _, l := range lines {
		it := domain.ShoppingItem{ID: auth.NewID(), Week: week, Name: l.Name, NormalizedName: l.NormalizedName, Quantity: l.Quantity, Unit: l.Unit, HaveQuantity: l.Have, ProductID: l.ProductID}
		if old, ok := prev[itemKey(l.NormalizedName, l.Unit, l.Quantity != nil)]; ok {
			it.ID = old.ID
			it.Checked = old.Checked && sameQty(old.Quantity, l.Quantity)
			it.Moved = old.Moved && sameQty(old.Quantity, l.Quantity)
		}
		fresh = append(fresh, it)
	}
	if err := s.repo.ReplaceGeneratedItems(ctx, householdID, week, fresh); err != nil {
		return nil, err
	}
	return s.repo.ShoppingItems(ctx, householdID, week)
}

func (s *Service) AddManual(ctx context.Context, householdID, week string, in ItemInput) (domain.ShoppingItem, error) {
	if _, err := plan.ValidWeek(week); err != nil {
		return domain.ShoppingItem{}, err
	}
	name, unit, err := in.validate()
	if err != nil {
		return domain.ShoppingItem{}, err
	}
	it := domain.ShoppingItem{ID: auth.NewID(), HouseholdID: householdID, Week: week, Name: name,
		NormalizedName: units.NormalizeName(name), Quantity: in.Quantity, Unit: unit, Manual: true}
	if err := s.repo.InsertShoppingItem(ctx, it); err != nil {
		return domain.ShoppingItem{}, err
	}
	return it, nil
}

func (s *Service) SetChecked(ctx context.Context, householdID, id string, checked bool) error {
	it, err := s.repo.ShoppingItemByID(ctx, householdID, id)
	if err != nil {
		return err
	}
	it.Checked = checked
	return s.repo.UpdateShoppingItem(ctx, it)
}

func (s *Service) Edit(ctx context.Context, householdID, id string, in ItemInput) (domain.ShoppingItem, error) {
	it, err := s.repo.ShoppingItemByID(ctx, householdID, id)
	if err != nil {
		return domain.ShoppingItem{}, err
	}
	if !it.Manual {
		return domain.ShoppingItem{}, invalid("generated items cannot be edited; change the plan or recipe instead")
	}
	name, unit, err := in.validate()
	if err != nil {
		return domain.ShoppingItem{}, err
	}
	it.Name, it.NormalizedName, it.Quantity, it.Unit = name, units.NormalizeName(name), in.Quantity, unit
	if err := s.repo.UpdateShoppingItem(ctx, it); err != nil {
		return domain.ShoppingItem{}, err
	}
	return it, nil
}

func (s *Service) Remove(ctx context.Context, householdID, id string) error {
	return s.repo.DeleteShoppingItem(ctx, householdID, id)
}

// MoveCheckedToPantry claims every checked, not yet moved item that has a quantity (atomically, so a
// second or concurrent call gets nothing) and adds each to the pantry. An item whose add fails is
// released again so a retry picks it up. Returns the number added and the first error. Covered lines
// (quantity 0) stay out; an item without a unit is stocked as pieces.
func (s *Service) MoveCheckedToPantry(ctx context.Context, householdID, week string) (int, error) {
	if _, err := plan.ValidWeek(week); err != nil {
		return 0, err
	}
	claimed, err := s.repo.ClaimShoppingItemsForMove(ctx, householdID, week)
	if err != nil {
		return 0, err
	}
	added := 0
	var firstErr error
	var failed []string
	for _, it := range claimed {
		unit := it.Unit
		if unit == "" {
			unit = "szt"
		}
		if _, err := s.pantry.Add(ctx, householdID, pantry.Input{ProductID: it.ProductID, Name: it.Name, Quantity: it.Quantity, Unit: unit}); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			failed = append(failed, it.ID)
			continue
		}
		added++
	}
	if len(failed) > 0 {
		if err := s.repo.UnclaimShoppingItems(ctx, householdID, failed); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return added, firstErr
}
