// Package recipe holds recipe use cases: validation, normalization and persistence calls.
package recipe

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/units"
)

const (
	maxTitle       = 200
	maxServings    = 100
	maxIngredients = 200
	maxSteps       = 100
	maxStepLen     = 2000
	maxTags        = 20
	maxTagLen      = 40
	maxUnitLen     = 30
	maxNote        = 200
	maxQuantity    = 1e6
	maxURL         = 2000
)

type Repo interface {
	CreateRecipe(ctx context.Context, r domain.Recipe) error
	UpdateRecipe(ctx context.Context, r domain.Recipe) error
	RecipeByID(ctx context.Context, householdID, id string) (domain.Recipe, error)
	ListRecipes(ctx context.Context, householdID string) ([]domain.Recipe, error)
	DeleteRecipe(ctx context.Context, householdID, id string) error
}

type IngredientInput struct {
	Name     string
	Quantity *float64
	Unit     string
	Note     string
}

type Input struct {
	Title        string
	BaseServings int
	SourceURL    string
	Tags         []string
	Ingredients  []IngredientInput
	Steps        []string
}

type Service struct {
	repo Repo
	now  func() time.Time
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

// checkText rejects NUL and other control characters (Postgres cannot store NUL) in every text field.
func checkText(in Input) error {
	bad := domain.HasControl(in.Title) || domain.HasControl(in.SourceURL)
	for _, ing := range in.Ingredients {
		bad = bad || domain.HasControl(ing.Name) || domain.HasControl(ing.Unit) || domain.HasControl(ing.Note)
	}
	for _, st := range in.Steps {
		bad = bad || domain.HasControl(st)
	}
	for _, tag := range in.Tags {
		bad = bad || domain.HasControl(tag)
	}
	if bad {
		return invalid("text must not contain control characters")
	}
	return nil
}

// build validates the input and returns a recipe with normalized fields (no ID, owner or times).
func build(in Input) (domain.Recipe, error) {
	if err := checkText(in); err != nil {
		return domain.Recipe{}, err
	}
	r := domain.Recipe{Title: strings.TrimSpace(in.Title), BaseServings: in.BaseServings, SourceURL: strings.TrimSpace(in.SourceURL)}
	if n := utf8.RuneCountInString(r.Title); n == 0 || n > maxTitle {
		return r, invalid("title must be 1 to %d characters", maxTitle)
	}
	if r.BaseServings < 1 || r.BaseServings > maxServings {
		return r, invalid("baseServings must be 1 to %d", maxServings)
	}
	if r.SourceURL != "" {
		u, err := url.Parse(r.SourceURL)
		if err != nil || len(r.SourceURL) > maxURL || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return r, invalid("sourceUrl must be an http or https address")
		}
	}
	if len(in.Ingredients) > maxIngredients {
		return r, invalid("at most %d ingredients", maxIngredients)
	}
	for _, ing := range in.Ingredients {
		name := strings.TrimSpace(ing.Name)
		unit := units.CanonUnit(ing.Unit)
		note := strings.TrimSpace(ing.Note)
		if n := utf8.RuneCountInString(name); n == 0 || n > maxTitle {
			return r, invalid("ingredient name must be 1 to %d characters", maxTitle)
		}
		if utf8.RuneCountInString(unit) > maxUnitLen {
			return r, invalid("unit must be at most %d characters", maxUnitLen)
		}
		if utf8.RuneCountInString(note) > maxNote {
			return r, invalid("ingredient note must be at most %d characters", maxNote)
		}
		if ing.Quantity != nil {
			v := *ing.Quantity
			if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > maxQuantity {
				return r, invalid("quantity must be greater than 0 and at most %v", maxQuantity)
			}
		}
		r.Ingredients = append(r.Ingredients, domain.Ingredient{Name: name, NormalizedName: units.NormalizeName(name), Quantity: ing.Quantity, Unit: unit, Note: note})
	}
	if len(in.Steps) > maxSteps {
		return r, invalid("at most %d steps", maxSteps)
	}
	for _, s := range in.Steps {
		s = strings.TrimSpace(s)
		if n := utf8.RuneCountInString(s); n == 0 || n > maxStepLen {
			return r, invalid("each step must be 1 to %d characters", maxStepLen)
		}
		r.Steps = append(r.Steps, s)
	}
	if len(in.Tags) > maxTags {
		return r, invalid("at most %d tags", maxTags)
	}
	seen := map[string]bool{}
	for _, tag := range in.Tags {
		tag = units.NormalizeName(tag)
		if n := utf8.RuneCountInString(tag); n == 0 || n > maxTagLen {
			return r, invalid("each tag must be 1 to %d characters", maxTagLen)
		}
		if !seen[tag] {
			seen[tag] = true
			r.Tags = append(r.Tags, tag)
		}
	}
	return r, nil
}

func (s *Service) Create(ctx context.Context, householdID string, in Input) (domain.Recipe, error) {
	r, err := build(in)
	if err != nil {
		return domain.Recipe{}, err
	}
	r.ID, r.HouseholdID = auth.NewID(), householdID
	r.CreatedAt = s.now().UTC()
	r.UpdatedAt = r.CreatedAt
	if err := s.repo.CreateRecipe(ctx, r); err != nil {
		return domain.Recipe{}, err
	}
	return r, nil
}

func (s *Service) Update(ctx context.Context, householdID, id string, in Input) (domain.Recipe, error) {
	r, err := build(in)
	if err != nil {
		return domain.Recipe{}, err
	}
	old, err := s.repo.RecipeByID(ctx, householdID, id)
	if err != nil {
		return domain.Recipe{}, err
	}
	r.ID, r.HouseholdID, r.CreatedAt = id, householdID, old.CreatedAt
	r.UpdatedAt = s.now().UTC()
	if err := s.repo.UpdateRecipe(ctx, r); err != nil {
		return domain.Recipe{}, err
	}
	return r, nil
}

func (s *Service) Get(ctx context.Context, householdID, id string) (domain.Recipe, error) {
	return s.repo.RecipeByID(ctx, householdID, id)
}

func (s *Service) List(ctx context.Context, householdID string) ([]domain.Recipe, error) {
	return s.repo.ListRecipes(ctx, householdID)
}

func (s *Service) Delete(ctx context.Context, householdID, id string) error {
	return s.repo.DeleteRecipe(ctx, householdID, id)
}
