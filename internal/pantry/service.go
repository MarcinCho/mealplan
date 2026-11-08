// Package pantry holds household stock: items, the monthly review and the stock changes made when
// planned meals are marked done.
package pantry

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/product"
	"mealplan/internal/store"
	"mealplan/internal/units"
)

const (
	maxQuantity = 1e6
	maxUnitLen  = 30
	dateLayout  = "2006-01-02"
)

type Repo interface {
	PantryItems(ctx context.Context, householdID string) ([]domain.PantryItem, error)
	PantryItemByID(ctx context.Context, householdID, id string) (domain.PantryItem, error)
	AddToPantry(ctx context.Context, householdID, productID, unit string, amount float64) (domain.PantryItem, error)
	SavePantryItem(ctx context.Context, it domain.PantryItem) error
	DeletePantryItem(ctx context.Context, householdID, id string) error
	ApplyMovements(ctx context.Context, householdID string, ms []domain.PantryMovement) error
	MovementsForEntry(ctx context.Context, householdID, entryID string) ([]domain.PantryMovement, error)
	DeleteMovementsForEntry(ctx context.Context, householdID, entryID string) error
	PantryReviewedAt(ctx context.Context, householdID string) (*time.Time, error)
	MarkPantryReviewed(ctx context.Context, householdID string, at time.Time) error
}

type RecipeLookup interface {
	RecipeByID(ctx context.Context, householdID, id string) (domain.Recipe, error)
}

// Products is the part of product.Service the pantry needs.
type Products interface {
	All(ctx context.Context, householdID string) ([]domain.Product, error)
	Create(ctx context.Context, householdID string, in product.Input) (domain.Product, error)
}

// Input is a pantry item as submitted. ProductID or Name is required; a name that matches a product
// (or alias) uses it, otherwise a product without macros is created.
type Input struct {
	ProductID, Name string
	Quantity        *float64
	Unit            string
	ExpiresOn       string
	Staple          bool
}

type Service struct {
	repo     Repo
	recipes  RecipeLookup
	products Products
	now      func() time.Time
}

func New(repo Repo, recipes RecipeLookup, products Products, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, recipes: recipes, products: products, now: now}
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

// normalize validates the stock fields and returns the quantity (nil for a staple) and unit in base units.
func normalize(in Input) (*float64, string, error) {
	if domain.HasControl(in.Unit) || domain.HasControl(in.ExpiresOn) {
		return nil, "", invalid("text must not contain control characters")
	}
	unit := units.CanonUnit(in.Unit)
	if unit == "" {
		if !in.Staple {
			return nil, "", invalid("unit is required")
		}
		unit = "szt"
	}
	if utf8.RuneCountInString(unit) > maxUnitLen {
		return nil, "", invalid("unit must be at most %d characters", maxUnitLen)
	}
	if in.ExpiresOn != "" {
		if d, err := time.Parse(dateLayout, in.ExpiresOn); err != nil || d.Format(dateLayout) != in.ExpiresOn {
			return nil, "", invalid("expiresOn must be a date as YYYY-MM-DD")
		}
	}
	if in.Staple {
		return nil, unit, nil
	}
	if in.Quantity == nil {
		return nil, "", invalid("quantity is required unless the item is a staple")
	}
	q := *in.Quantity
	if math.IsNaN(q) || q <= 0 || q > maxQuantity {
		return nil, "", invalid("quantity must be greater than 0 and at most %v", maxQuantity)
	}
	base, unit := units.ToBase(q, unit)
	base = units.Round(base)
	if base <= 0 || base > maxQuantity {
		return nil, "", invalid("quantity must be greater than 0 and at most %v", maxQuantity)
	}
	return &base, unit, nil
}

// resolveProduct finds the product by ID (household-checked) or by name/alias, creating it if unknown.
func (s *Service) resolveProduct(ctx context.Context, householdID string, in Input) (domain.Product, error) {
	all, err := s.products.All(ctx, householdID)
	if err != nil {
		return domain.Product{}, err
	}
	if in.ProductID != "" {
		for _, p := range all {
			if p.ID == in.ProductID {
				return p, nil
			}
		}
		return domain.Product{}, store.ErrNotFound
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Product{}, invalid("productId or name is required")
	}
	if domain.HasControl(name) {
		return domain.Product{}, invalid("text must not contain control characters")
	}
	if p, ok := nutrition.NewMatcher(all).Match(domain.Ingredient{Name: name}); ok {
		return p, nil
	}
	return s.products.Create(ctx, householdID, product.Input{Name: name})
}

func (s *Service) List(ctx context.Context, householdID string) ([]domain.PantryItem, *time.Time, error) {
	items, err := s.repo.PantryItems(ctx, householdID)
	if err != nil {
		return nil, nil, err
	}
	at, err := s.repo.PantryReviewedAt(ctx, householdID)
	if err != nil {
		return nil, nil, err
	}
	return items, at, nil
}

func (s *Service) Add(ctx context.Context, householdID string, in Input) (domain.PantryItem, error) {
	qty, unit, err := normalize(in)
	if err != nil {
		return domain.PantryItem{}, err
	}
	p, err := s.resolveProduct(ctx, householdID, in)
	if err != nil {
		return domain.PantryItem{}, err
	}
	amount := 0.0
	if qty != nil {
		amount = *qty
	}
	it, err := s.repo.AddToPantry(ctx, householdID, p.ID, unit, amount)
	if err != nil {
		return domain.PantryItem{}, err
	}
	if in.Staple || in.ExpiresOn != "" {
		if in.Staple {
			it.Quantity, it.Staple = nil, true
		}
		if in.ExpiresOn != "" {
			it.ExpiresOn = in.ExpiresOn
		}
		if err := s.repo.SavePantryItem(ctx, it); err != nil {
			return domain.PantryItem{}, err
		}
		return s.repo.PantryItemByID(ctx, householdID, it.ID)
	}
	return it, nil
}

// Update replaces quantity, unit, expiry and staple flag of an item; its product cannot change.
func (s *Service) Update(ctx context.Context, householdID, id string, in Input) (domain.PantryItem, error) {
	qty, unit, err := normalize(in)
	if err != nil {
		return domain.PantryItem{}, err
	}
	it, err := s.repo.PantryItemByID(ctx, householdID, id)
	if err != nil {
		return domain.PantryItem{}, err
	}
	it.Quantity, it.Unit, it.ExpiresOn, it.Staple = qty, unit, in.ExpiresOn, in.Staple
	if err := s.repo.SavePantryItem(ctx, it); err != nil {
		return domain.PantryItem{}, err
	}
	return s.repo.PantryItemByID(ctx, householdID, id)
}

func (s *Service) Delete(ctx context.Context, householdID, id string) error {
	return s.repo.DeletePantryItem(ctx, householdID, id)
}

func (s *Service) MarkReviewed(ctx context.Context, householdID string) (time.Time, error) {
	at := s.now().UTC()
	return at, s.repo.MarkPantryReviewed(ctx, householdID, at)
}
