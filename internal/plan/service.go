package plan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/store"
)

const (
	maxFreeText = 300
	maxServings = 100
	maxOutValue = 10000
)

type Repo interface {
	InsertPlanEntry(ctx context.Context, e domain.PlanEntry) error
	UpdatePlanEntry(ctx context.Context, e domain.PlanEntry) error
	DeletePlanEntry(ctx context.Context, householdID, id string) error
	PlanEntryByID(ctx context.Context, householdID, id string) (domain.PlanEntry, error)
	PlanEntries(ctx context.Context, householdID, fromDate, toDate string) ([]domain.PlanEntry, error)
	CopyPlanEntries(ctx context.Context, householdID string, entries []domain.PlanEntry) error
}

type RecipeLookup interface {
	RecipeByID(ctx context.Context, householdID, id string) (domain.Recipe, error)
}

type EntryInput struct {
	Date       string
	Slot       domain.Slot
	RecipeID   string
	FreeText   string
	Servings   int // 0 = the recipe's base servings (1 for free text)
	CookUserID string
	Done       bool

	Kind          domain.EntryKind // empty = meal
	PrepDishID    string           // prep: exactly one of PrepDishID and FreezerItemID
	FreezerItemID string
	OutValues     domain.Macros // out: eaten-out values, nil = unknown
}

// Portions is the meal prep accounting a prep entry draws on (implemented by mealprep.Service).
type Portions interface {
	Available(ctx context.Context, householdID, dishID, freezerItemID, exceptEntryID string) (recipeID, label string, remaining int, err error)
	PortionsPerMeal(ctx context.Context, householdID string) (int, error)
}

// DoneHook is told when a plan entry is marked done or un-done (implemented by the pantry).
type DoneHook interface {
	EntryDone(ctx context.Context, householdID string, e domain.PlanEntry) error
	EntryUndone(ctx context.Context, householdID, entryID string) error
}

type Service struct {
	repo     Repo
	recipes  RecipeLookup
	hook     DoneHook
	portions Portions
}

// SetPortions registers the meal prep accounting; without it prep entries are refused.
func (s *Service) SetPortions(p Portions) { s.portions = p }

// ErrPantry means the entry was saved but the pantry could not be updated; retrying the save repairs it.
var ErrPantry = errors.New("plan entry saved, but updating the pantry failed")

// syncPantry makes stock match the saved entry's Done flag. The hook is state-based and idempotent,
// so repeating a save (e.g. after a failure) converges.
func (s *Service) syncPantry(ctx context.Context, e domain.PlanEntry) error {
	if s.hook == nil {
		return nil
	}
	var err error
	if e.Done {
		err = s.hook.EntryDone(ctx, e.HouseholdID, e)
	} else {
		err = s.hook.EntryUndone(ctx, e.HouseholdID, e.ID)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPantry, err)
	}
	return nil
}

// SetDoneHook registers the hook called after Edit changes an entry's Done flag.
func (s *Service) SetDoneHook(h DoneHook) { s.hook = h }

func New(repo Repo, recipes RecipeLookup) *Service { return &Service{repo: repo, recipes: recipes} }

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

func validOut(v *float64) bool { return v == nil || (!math.IsNaN(*v) && *v >= 0 && *v <= maxOutValue) }

// entry validates the input against the week and resolves defaults. cur is the stored entry when
// editing (its kind and source are kept), nil when adding.
func (s *Service) entry(ctx context.Context, householdID, week string, in EntryInput, cur *domain.PlanEntry) (domain.PlanEntry, error) {
	dates, err := WeekDates(week)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	inWeek := false
	for _, d := range dates {
		if d == in.Date {
			inWeek = true
		}
	}
	if !inWeek {
		return domain.PlanEntry{}, invalid("date must fall inside the week starting %s", week)
	}
	if !in.Slot.Valid() {
		return domain.PlanEntry{}, invalid("slot must be breakfast, lunch, dinner or snack")
	}
	if domain.HasControl(in.FreeText) {
		return domain.PlanEntry{}, invalid("freeText must not contain control characters")
	}
	text := strings.TrimSpace(in.FreeText)
	if utf8.RuneCountInString(text) > maxFreeText {
		return domain.PlanEntry{}, invalid("freeText must be at most %d characters", maxFreeText)
	}
	if in.Servings < 0 || in.Servings > maxServings {
		return domain.PlanEntry{}, invalid("servings must be 1 to %d", maxServings)
	}
	kind, dishID, freezerID, except := in.Kind, in.PrepDishID, in.FreezerItemID, ""
	if cur != nil {
		curKind := cur.Kind
		if curKind == "" {
			curKind = domain.EntryMeal
		}
		if (kind != "" && kind != curKind) || (dishID != "" && dishID != cur.PrepDishID) || (freezerID != "" && freezerID != cur.FreezerItemID) {
			return domain.PlanEntry{}, invalid("kind and source of an entry cannot be changed")
		}
		kind, dishID, freezerID, except = curKind, cur.PrepDishID, cur.FreezerItemID, cur.ID
	}
	if kind == "" {
		kind = domain.EntryMeal
	}
	base := domain.PlanEntry{HouseholdID: householdID, Date: in.Date, Slot: in.Slot, CookUserID: in.CookUserID, Done: in.Done, Kind: kind}
	switch kind {
	case domain.EntryMeal:
		if dishID != "" || freezerID != "" {
			return domain.PlanEntry{}, invalid("a meal has no prep source")
		}
		if (in.RecipeID == "") == (text == "") {
			return domain.PlanEntry{}, invalid("provide exactly one of recipeId and freeText")
		}
		servings := in.Servings
		if in.RecipeID != "" {
			r, err := s.recipes.RecipeByID(ctx, householdID, in.RecipeID)
			if err != nil {
				return domain.PlanEntry{}, err
			}
			if servings == 0 {
				servings = r.BaseServings
			}
		} else if servings == 0 {
			servings = 1
		}
		base.RecipeID, base.FreeText, base.Servings = in.RecipeID, text, servings
	case domain.EntryPrep:
		base.CookUserID = "" // portions are already cooked: no cook
		if s.portions == nil {
			return domain.PlanEntry{}, invalid("meal prep is not available")
		}
		if (dishID == "") == (freezerID == "") {
			return domain.PlanEntry{}, invalid("provide exactly one of prepDishId and freezerItemId")
		}
		recipeID, label, remaining, err := s.portions.Available(ctx, householdID, dishID, freezerID, except)
		if err != nil {
			return domain.PlanEntry{}, err
		}
		servings := in.Servings
		if servings == 0 {
			if servings, err = s.portions.PortionsPerMeal(ctx, householdID); err != nil {
				return domain.PlanEntry{}, err
			}
		}
		if servings > remaining {
			return domain.PlanEntry{}, invalid("only %d portions left", remaining)
		}
		base.PrepDishID, base.FreezerItemID, base.Servings = dishID, freezerID, servings
		if recipeID != "" {
			base.RecipeID = recipeID
		} else {
			base.FreeText = label
		}
	case domain.EntryOut:
		base.CookUserID = "" // eaten out: no cook
		if in.RecipeID != "" || dishID != "" || freezerID != "" {
			return domain.PlanEntry{}, invalid("an eaten-out entry has no recipe or prep source")
		}
		for _, v := range []*float64{in.OutValues.Kcal, in.OutValues.Protein, in.OutValues.Carbs, in.OutValues.Fat} {
			if !validOut(v) {
				return domain.PlanEntry{}, invalid("values must be 0 to %d", maxOutValue)
			}
		}
		base.FreeText, base.Servings, base.OutValues = text, 1, in.OutValues
	default:
		return domain.PlanEntry{}, invalid("kind must be meal, prep or out")
	}
	return base, nil
}

func (s *Service) Week(ctx context.Context, householdID, week string) ([]domain.PlanEntry, error) {
	dates, err := WeekDates(week)
	if err != nil {
		return nil, err
	}
	return s.repo.PlanEntries(ctx, householdID, dates[0], dates[6])
}

func (s *Service) Add(ctx context.Context, householdID, week string, in EntryInput) (domain.PlanEntry, error) {
	e, err := s.entry(ctx, householdID, week, in, nil)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	e.ID = auth.NewID()
	if err := s.repo.InsertPlanEntry(ctx, e); err != nil {
		return domain.PlanEntry{}, err
	}
	if err := s.syncPantry(ctx, e); err != nil {
		return e, err
	}
	return e, nil
}

// inWeek returns the current entry, or store.ErrNotFound unless the household's entry exists and is dated inside the week.
func (s *Service) inWeek(ctx context.Context, householdID, week, id string) (domain.PlanEntry, error) {
	dates, err := WeekDates(week)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	cur, err := s.repo.PlanEntryByID(ctx, householdID, id)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	if cur.Date < dates[0] || cur.Date > dates[6] {
		return domain.PlanEntry{}, store.ErrNotFound
	}
	return cur, nil
}

func (s *Service) Edit(ctx context.Context, householdID, week, id string, in EntryInput) (domain.PlanEntry, error) {
	cur, err := s.inWeek(ctx, householdID, week, id)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	e, err := s.entry(ctx, householdID, week, in, &cur)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	e.ID = id
	if err := s.repo.UpdatePlanEntry(ctx, e); err != nil {
		return domain.PlanEntry{}, err
	}
	if err := s.syncPantry(ctx, e); err != nil {
		return e, err
	}
	return e, nil
}

func (s *Service) Remove(ctx context.Context, householdID, week, id string) error {
	if _, err := s.inWeek(ctx, householdID, week, id); err != nil {
		return err
	}
	return s.repo.DeletePlanEntry(ctx, householdID, id)
}

// Copy duplicates every entry of fromWeek into toWeek, shifted by whole weeks, and returns how many
// entries it created. The target week must be empty.
func (s *Service) Copy(ctx context.Context, householdID, fromWeek, toWeek string) (int, error) {
	from, err := ValidWeek(fromWeek)
	if err != nil {
		return 0, err
	}
	to, err := ValidWeek(toWeek)
	if err != nil {
		return 0, err
	}
	if fromWeek == toWeek {
		return 0, invalid("source and target week must differ")
	}
	src, err := s.Week(ctx, householdID, fromWeek)
	if err != nil {
		return 0, err
	}
	existing, err := s.Week(ctx, householdID, toWeek)
	if err != nil {
		return 0, err
	}
	if len(existing) > 0 {
		return 0, store.ErrConflict
	}
	shift := int(to.Sub(from).Hours() / 24)
	out := make([]domain.PlanEntry, 0, len(src))
	for _, e := range src {
		if e.Kind == domain.EntryPrep {
			continue
		}
		d, _ := time.Parse(dateLayout, e.Date)
		e.ID = auth.NewID()
		e.Date = d.AddDate(0, 0, shift).Format(dateLayout)
		e.Done = false
		out = append(out, e)
	}
	if err := s.repo.CopyPlanEntries(ctx, householdID, out); err != nil {
		return 0, err
	}
	return len(out), nil
}
