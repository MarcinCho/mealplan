// Package product holds the household product database: validation, alias rules and lazy seeding.
package product

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/nutrition/seed"
	"mealplan/internal/store"
	"mealplan/internal/units"
)

const (
	maxName     = 200
	maxAliases  = 20
	maxAliasLen = 200
	maxKcal     = 900
	maxMacro    = 100
	maxPiece    = 5000
)

type Repo interface {
	CreateProduct(ctx context.Context, p domain.Product) error
	UpdateProduct(ctx context.Context, p domain.Product) error
	DeleteProduct(ctx context.Context, householdID, id string) error
	ProductByID(ctx context.Context, householdID, id string) (domain.Product, error)
	ListProducts(ctx context.Context, householdID string) ([]domain.Product, error)
	SeedProducts(ctx context.Context, householdID string, ps []domain.Product) (bool, error)
}

type Input struct {
	Name          string
	Per100g       domain.Macros
	PieceGrams    *float64
	DensityGPerML *float64
	Aliases       []string
}

type Service struct {
	repo   Repo
	now    func() time.Time
	seeded sync.Map // household IDs already seeded by this process
}

func New(repo Repo, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

func badNum(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) }

func checkMacros(m domain.Macros) error {
	sum := 0.0
	for _, f := range []struct {
		name string
		v    *float64
		max  float64
	}{{"kcal", m.Kcal, maxKcal}, {"protein", m.Protein, maxMacro}, {"carbs", m.Carbs, maxMacro}, {"fat", m.Fat, maxMacro}} {
		if f.v == nil {
			continue
		}
		if badNum(*f.v) || *f.v < 0 || *f.v > f.max {
			return invalid("%s must be between 0 and %v per 100 g", f.name, f.max)
		}
		if f.name != "kcal" {
			sum += *f.v
		}
	}
	if sum > 100 {
		return invalid("protein, carbs and fat must add up to at most 100 g per 100 g")
	}
	return nil
}

// build validates the input and returns a product with normalized name and aliases (no ID, owner or times).
func build(in Input) (domain.Product, error) {
	name := strings.TrimSpace(in.Name)
	bad := domain.HasControl(in.Name)
	for _, a := range in.Aliases {
		bad = bad || domain.HasControl(a)
	}
	if bad {
		return domain.Product{}, invalid("text must not contain control characters")
	}
	if n := utf8.RuneCountInString(name); n == 0 || n > maxName {
		return domain.Product{}, invalid("name must be 1 to %d characters", maxName)
	}
	p := domain.Product{Name: name, NormalizedName: units.NormalizeName(name), Per100g: in.Per100g, PieceGrams: in.PieceGrams, DensityGPerML: in.DensityGPerML}
	if err := checkMacros(in.Per100g); err != nil {
		return p, err
	}
	if v := in.PieceGrams; v != nil && (badNum(*v) || *v <= 0 || *v > maxPiece) {
		return p, invalid("pieceGrams must be greater than 0 and at most %d", maxPiece)
	}
	if v := in.DensityGPerML; v != nil && (badNum(*v) || *v <= 0.2 || *v > 2) {
		return p, invalid("densityGPerMl must be greater than 0.2 and at most 2")
	}
	if len(in.Aliases) > maxAliases {
		return p, invalid("at most %d aliases", maxAliases)
	}
	seen := map[string]bool{p.NormalizedName: true}
	for _, a := range in.Aliases {
		a = units.NormalizeName(a)
		if n := utf8.RuneCountInString(a); n == 0 || n > maxAliasLen {
			return p, invalid("each alias must be 1 to %d characters", maxAliasLen)
		}
		if !seen[a] {
			seen[a] = true
			p.Aliases = append(p.Aliases, a)
		}
	}
	return p, nil
}

// checkAliases refuses aliases that equal another product's name in the household.
func (s *Service) checkAliases(ctx context.Context, p domain.Product) error {
	if len(p.Aliases) == 0 {
		return nil
	}
	list, err := s.repo.ListProducts(ctx, p.HouseholdID)
	if err != nil {
		return err
	}
	names := make(map[string]bool, len(list))
	for _, o := range list {
		if o.ID != p.ID {
			names[o.NormalizedName] = true
		}
	}
	for _, a := range p.Aliases {
		if names[a] {
			return fmt.Errorf("%w: alias %q is the name of another product", store.ErrConflict, a)
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, householdID string, in Input) (domain.Product, error) {
	p, err := build(in)
	if err != nil {
		return domain.Product{}, err
	}
	p.ID, p.HouseholdID = auth.NewID(), householdID
	p.CreatedAt = s.now().UTC()
	p.UpdatedAt = p.CreatedAt
	if err := s.checkAliases(ctx, p); err != nil {
		return domain.Product{}, err
	}
	if err := s.repo.CreateProduct(ctx, p); err != nil {
		return domain.Product{}, err
	}
	return p, nil
}

func (s *Service) Update(ctx context.Context, householdID, id string, in Input) (domain.Product, error) {
	p, err := build(in)
	if err != nil {
		return domain.Product{}, err
	}
	old, err := s.repo.ProductByID(ctx, householdID, id)
	if err != nil {
		return domain.Product{}, err
	}
	p.ID, p.HouseholdID, p.CreatedAt = id, householdID, old.CreatedAt
	p.UpdatedAt = s.now().UTC()
	if err := s.checkAliases(ctx, p); err != nil {
		return domain.Product{}, err
	}
	if err := s.repo.UpdateProduct(ctx, p); err != nil {
		return domain.Product{}, err
	}
	return p, nil
}

func (s *Service) Delete(ctx context.Context, householdID, id string) error {
	return s.repo.DeleteProduct(ctx, householdID, id)
}

// ensureSeeded installs the bundled products the first time a household is read; the store
// remembers it, so a household that deleted everything is not seeded again.
func (s *Service) ensureSeeded(ctx context.Context, householdID string) error {
	if _, ok := s.seeded.Load(householdID); ok {
		return nil
	}
	ps, err := seed.Products()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	for i := range ps {
		ps[i].ID, ps[i].HouseholdID, ps[i].CreatedAt, ps[i].UpdatedAt = auth.NewID(), householdID, now, now
	}
	if _, err := s.repo.SeedProducts(ctx, householdID, ps); err != nil {
		return err
	}
	s.seeded.Store(householdID, true)
	return nil
}

// All returns every product of the household, seeding the bundled ones on first use.
func (s *Service) All(ctx context.Context, householdID string) ([]domain.Product, error) {
	if err := s.ensureSeeded(ctx, householdID); err != nil {
		return nil, err
	}
	return s.repo.ListProducts(ctx, householdID)
}

// List is All filtered by q (case-insensitive substring of the name or an alias).
func (s *Service) List(ctx context.Context, householdID, q string) ([]domain.Product, error) {
	all, err := s.All(ctx, householdID)
	if err != nil {
		return nil, err
	}
	q = units.NormalizeName(q)
	if q == "" {
		return all, nil
	}
	out := make([]domain.Product, 0)
	for _, p := range all {
		match := strings.Contains(p.NormalizedName, q)
		for _, a := range p.Aliases {
			match = match || strings.Contains(a, q)
		}
		if match {
			out = append(out, p)
		}
	}
	return out, nil
}
