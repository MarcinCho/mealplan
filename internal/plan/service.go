package plan

import (
	"context"
	"fmt"
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
}

// DoneHook is told when a plan entry is marked done or un-done (implemented by the pantry).
type DoneHook interface {
	EntryDone(ctx context.Context, householdID string, e domain.PlanEntry) error
	EntryUndone(ctx context.Context, householdID, entryID string) error
}

type Service struct {
	repo    Repo
	recipes RecipeLookup
	hook    DoneHook
}

// SetDoneHook registers the hook called after Edit changes an entry's Done flag.
func (s *Service) SetDoneHook(h DoneHook) { s.hook = h }

func New(repo Repo, recipes RecipeLookup) *Service { return &Service{repo: repo, recipes: recipes} }

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

// entry validates the input against the week and resolves defaults.
func (s *Service) entry(ctx context.Context, householdID, week string, in EntryInput) (domain.PlanEntry, error) {
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
	if (in.RecipeID == "") == (text == "") {
		return domain.PlanEntry{}, invalid("provide exactly one of recipeId and freeText")
	}
	if utf8.RuneCountInString(text) > maxFreeText {
		return domain.PlanEntry{}, invalid("freeText must be at most %d characters", maxFreeText)
	}
	if in.Servings < 0 || in.Servings > maxServings {
		return domain.PlanEntry{}, invalid("servings must be 1 to %d", maxServings)
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
	return domain.PlanEntry{HouseholdID: householdID, Date: in.Date, Slot: in.Slot, RecipeID: in.RecipeID,
		FreeText: text, Servings: servings, CookUserID: in.CookUserID, Done: in.Done}, nil
}

func (s *Service) Week(ctx context.Context, householdID, week string) ([]domain.PlanEntry, error) {
	dates, err := WeekDates(week)
	if err != nil {
		return nil, err
	}
	return s.repo.PlanEntries(ctx, householdID, dates[0], dates[6])
}

func (s *Service) Add(ctx context.Context, householdID, week string, in EntryInput) (domain.PlanEntry, error) {
	e, err := s.entry(ctx, householdID, week, in)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	e.ID = auth.NewID()
	if err := s.repo.InsertPlanEntry(ctx, e); err != nil {
		return domain.PlanEntry{}, err
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
	e, err := s.entry(ctx, householdID, week, in)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	cur, err := s.inWeek(ctx, householdID, week, id)
	if err != nil {
		return domain.PlanEntry{}, err
	}
	e.ID = id
	if err := s.repo.UpdatePlanEntry(ctx, e); err != nil {
		return domain.PlanEntry{}, err
	}
	if s.hook != nil && cur.Done != e.Done {
		var herr error
		if e.Done {
			herr = s.hook.EntryDone(ctx, householdID, e)
		} else {
			herr = s.hook.EntryUndone(ctx, householdID, id)
		}
		if herr != nil {
			return e, fmt.Errorf("plan entry saved, but updating the pantry failed: %w", herr)
		}
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
