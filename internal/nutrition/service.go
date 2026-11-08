// Package nutrition computes nutrition values and holds the per-account tracking use cases.
package nutrition

import (
	"context"
	"fmt"
	"math"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
)

const (
	dateLayout = "2006-01-02"
	maxDays    = 31
	maxKcal    = 20000
	maxMacro   = 2000
	maxQty     = 10000
)

type Repo interface {
	NutritionSettings(ctx context.Context, householdID, userID string) (domain.NutritionSettings, error)
	SaveNutritionSettings(ctx context.Context, s domain.NutritionSettings) error
	AddFoodLog(ctx context.Context, e domain.FoodLogEntry) error
	DeleteFoodLog(ctx context.Context, householdID, userID, id string) error
	FoodLog(ctx context.Context, householdID, userID, from, to string) ([]domain.FoodLogEntry, error)
	ProductByID(ctx context.Context, householdID, id string) (domain.Product, error)
	PlanEntries(ctx context.Context, householdID, fromDate, toDate string) ([]domain.PlanEntry, error)
	RecipesByIDs(ctx context.Context, householdID string, ids []string) (map[string]domain.Recipe, error)
}

type Products interface {
	All(ctx context.Context, householdID string) ([]domain.Product, error)
}

type Service struct {
	repo     Repo
	products Products
	now      func() time.Time
}

func New(repo Repo, products Products, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, products: products, now: now}
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Format(dateLayout) != s {
		return time.Time{}, invalid("date must be a YYYY-MM-DD date")
	}
	return t, nil
}

func (s *Service) Settings(ctx context.Context, p auth.Principal) (domain.NutritionSettings, error) {
	return s.repo.NutritionSettings(ctx, p.HouseholdID, p.UserID)
}

func (s *Service) SaveSettings(ctx context.Context, p auth.Principal, enabled bool, goal domain.Macros) error {
	for _, c := range []struct {
		name string
		v    *float64
		max  float64
	}{{"kcal", goal.Kcal, maxKcal}, {"protein", goal.Protein, maxMacro}, {"carbs", goal.Carbs, maxMacro}, {"fat", goal.Fat, maxMacro}} {
		if c.v != nil && (math.IsNaN(*c.v) || *c.v < 0 || *c.v > c.max) {
			return invalid("goal %s must be between 0 and %g", c.name, c.max)
		}
	}
	return s.repo.SaveNutritionSettings(ctx, domain.NutritionSettings{UserID: p.UserID, HouseholdID: p.HouseholdID, Enabled: enabled, Goal: goal})
}

func (s *Service) AddLog(ctx context.Context, p auth.Principal, date, productID string, quantity float64, unit string) (domain.FoodLogEntry, error) {
	if _, err := parseDate(date); err != nil {
		return domain.FoodLogEntry{}, err
	}
	if unit != "g" && unit != "szt" {
		return domain.FoodLogEntry{}, invalid("unit must be g or szt")
	}
	if math.IsNaN(quantity) || quantity <= 0 || quantity > maxQty {
		return domain.FoodLogEntry{}, invalid("quantity must be greater than 0 and at most %d", maxQty)
	}
	if _, err := s.repo.ProductByID(ctx, p.HouseholdID, productID); err != nil {
		return domain.FoodLogEntry{}, err // store.ErrNotFound for unknown and foreign products
	}
	e := domain.FoodLogEntry{ID: auth.NewID(), HouseholdID: p.HouseholdID, UserID: p.UserID, Date: date,
		ProductID: productID, Quantity: quantity, Unit: unit, CreatedAt: s.now()}
	if err := s.repo.AddFoodLog(ctx, e); err != nil {
		return domain.FoodLogEntry{}, err
	}
	return e, nil
}

func (s *Service) DeleteLog(ctx context.Context, p auth.Principal, id string) error {
	return s.repo.DeleteFoodLog(ctx, p.HouseholdID, p.UserID, id)
}

// Days returns one summary per date in [from, to] (at most 31 days); an
// account that has tracking disabled gets none.
func (s *Service) Days(ctx context.Context, p auth.Principal, from, to string) ([]DaySummary, error) {
	start, err := parseDate(from)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(to)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, invalid("to must not be before from")
	}
	if end.Sub(start) > (maxDays-1)*24*time.Hour {
		return nil, invalid("range must be at most %d days", maxDays)
	}
	set, err := s.repo.NutritionSettings(ctx, p.HouseholdID, p.UserID)
	if err != nil {
		return nil, err
	}
	if !set.Enabled {
		return []DaySummary{}, nil
	}
	plan, err := s.repo.PlanEntries(ctx, p.HouseholdID, from, to)
	if err != nil {
		return nil, err
	}
	logs, err := s.repo.FoodLog(ctx, p.HouseholdID, p.UserID, from, to)
	if err != nil {
		return nil, err
	}
	prods, err := s.products.All(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	var ids []string
	seen := map[string]bool{}
	for _, e := range plan {
		if e.RecipeID != "" && !seen[e.RecipeID] {
			seen[e.RecipeID] = true
			ids = append(ids, e.RecipeID)
		}
	}
	recipes, err := s.repo.RecipesByIDs(ctx, p.HouseholdID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.Product, len(prods))
	for _, pr := range prods {
		byID[pr.ID] = pr
	}
	m := NewMatcher(prods)
	planBy := map[string][]domain.PlanEntry{}
	for _, e := range plan {
		planBy[e.Date] = append(planBy[e.Date], e)
	}
	logBy := map[string][]domain.FoodLogEntry{}
	for _, l := range logs {
		logBy[l.Date] = append(logBy[l.Date], l)
	}
	var out []DaySummary
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		date := d.Format(dateLayout)
		out = append(out, BuildDay(date, set.Goal, planBy[date], recipes, logBy[date], byID, m))
	}
	return out, nil
}
