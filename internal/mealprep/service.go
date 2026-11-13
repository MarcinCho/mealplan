// Package mealprep holds cooking sessions, the dishes cooked in them, the freezer and the portion
// accounting that plan entries drawing on them rely on.
package mealprep

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/plan"
	"mealplan/internal/store"
)

const (
	dateLayout  = "2006-01-02"
	maxName     = 100
	maxPortions = 200
	maxPerMeal  = 20
	maxLabelLen = 100
)

// ErrPantry means the session was saved, but updating the pantry failed; saving again repairs it.
var ErrPantry = errors.New("prep session saved, but updating the pantry failed")

type Repo interface {
	InsertPrepSession(ctx context.Context, p domain.PrepSession) error
	UpdatePrepSession(ctx context.Context, p domain.PrepSession) error
	DeletePrepSession(ctx context.Context, householdID, id string) error
	PrepSessionByID(ctx context.Context, householdID, id string) (domain.PrepSession, error)
	PrepSessions(ctx context.Context, householdID string) ([]domain.PrepSession, error)
	InsertPrepDish(ctx context.Context, d domain.PrepDish) error
	UpdatePrepDish(ctx context.Context, d domain.PrepDish) error
	DeletePrepDish(ctx context.Context, householdID, id string) error
	PrepDishByID(ctx context.Context, householdID, id string) (domain.PrepDish, error)
	PrepDishes(ctx context.Context, householdID string, sessionIDs []string) ([]domain.PrepDish, error)
	InsertFreezerItem(ctx context.Context, f domain.FreezerItem) error
	UpdateFreezerItem(ctx context.Context, f domain.FreezerItem) error
	DeleteFreezerItem(ctx context.Context, householdID, id string) error
	FreezerItemByID(ctx context.Context, householdID, id string) (domain.FreezerItem, error)
	FreezerItems(ctx context.Context, householdID string) ([]domain.FreezerItem, error)
	Allocations(ctx context.Context, householdID string) ([]domain.PlanEntry, error)
	PortionsPerMeal(ctx context.Context, householdID string) (int, error)
	SetPortionsPerMeal(ctx context.Context, householdID string, n int) error
}

type RecipeLookup interface {
	RecipeByID(ctx context.Context, householdID, id string) (domain.Recipe, error)
}

// CookHook is told when a session is marked cooked or un-cooked (implemented by the pantry).
type CookHook interface {
	SessionCooked(ctx context.Context, householdID string, s domain.PrepSession, dishes []domain.PrepDish) error
	SessionUncooked(ctx context.Context, householdID, sessionID string) error
}

// SessionInput is a session as submitted. An empty Week means DefaultWeek(Date).
type SessionInput struct {
	Date, Name, Week string
	Cooked           bool // used by UpdateSession only
}

// FreezerInput is a freezer item as submitted; Label defaults to the recipe's title.
type FreezerInput struct {
	RecipeID, Label string
	Portions        int
	FrozenOn        string
}

type DishView struct {
	domain.PrepDish
	RecipeTitle                  string
	Allocated, Frozen, Remaining int
	Allocations                  []domain.PlanEntry
}

type SessionView struct {
	domain.PrepSession
	Dishes    []DishView
	Remaining int
}

type FreezerView struct {
	domain.FreezerItem
	Title                string
	Allocated, Remaining int
	Allocations          []domain.PlanEntry
}

type Service struct {
	repo    Repo
	recipes RecipeLookup
	hook    CookHook
	now     func() time.Time
}

func New(repo Repo, recipes RecipeLookup, hook CookHook, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, recipes: recipes, hook: hook, now: now}
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

func validDate(s string) error {
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Format(dateLayout) != s {
		return invalid("date must be YYYY-MM-DD")
	}
	return nil
}

// DefaultWeek is the Monday of the shopping week a session on date belongs to: Monday to Friday
// belong to their own week, a weekend session shops for the coming week.
func DefaultWeek(date string) (string, error) {
	if err := validDate(date); err != nil {
		return "", err
	}
	t, _ := time.Parse(dateLayout, date)
	switch t.Weekday() {
	case time.Saturday:
		t = t.AddDate(0, 0, 2)
	case time.Sunday:
		t = t.AddDate(0, 0, 1)
	default:
		t = t.AddDate(0, 0, -int(t.Weekday()-time.Monday))
	}
	return t.Format(dateLayout), nil
}

func (s *Service) checkSession(in SessionInput) (name, week string, err error) {
	name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxName || domain.HasControl(name) {
		return "", "", invalid("name must be 1-%d characters without control characters", maxName)
	}
	if err := validDate(in.Date); err != nil {
		return "", "", err
	}
	week = in.Week
	if week == "" {
		week, err = DefaultWeek(in.Date)
	} else {
		_, err = plan.ValidWeek(week)
	}
	return name, week, err
}

func checkPortions(n int) error {
	if n < 1 || n > maxPortions {
		return invalid("portions must be 1-%d", maxPortions)
	}
	return nil
}

func (s *Service) title(ctx context.Context, hh, recipeID string, cache map[string]string) (string, error) {
	if recipeID == "" {
		return "", nil
	}
	if t, ok := cache[recipeID]; ok {
		return t, nil
	}
	r, err := s.recipes.RecipeByID(ctx, hh, recipeID)
	if errors.Is(err, store.ErrNotFound) {
		cache[recipeID] = ""
		return "", nil
	}
	if err != nil {
		return "", err
	}
	cache[recipeID] = r.Title
	return r.Title, nil
}

// views builds the session views for the given sessions.
func (s *Service) views(ctx context.Context, hh string, sessions []domain.PrepSession) ([]SessionView, error) {
	ids := make([]string, len(sessions))
	for i, p := range sessions {
		ids[i] = p.ID
	}
	dishes, err := s.repo.PrepDishes(ctx, hh, ids)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.FreezerItems(ctx, hh)
	if err != nil {
		return nil, err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	bySession := map[string][]DishView{}
	for _, d := range dishes {
		t, err := s.title(ctx, hh, d.RecipeID, titles)
		if err != nil {
			return nil, err
		}
		v := DishView{PrepDish: d, RecipeTitle: t, Allocated: DishAllocated(d.ID, allocs), Frozen: DishFrozen(d.ID, items),
			Remaining: DishRemaining(d, items, allocs), Allocations: []domain.PlanEntry{}}
		for _, e := range allocs {
			if e.PrepDishID == d.ID {
				v.Allocations = append(v.Allocations, e)
			}
		}
		bySession[d.SessionID] = append(bySession[d.SessionID], v)
	}
	out := make([]SessionView, len(sessions))
	for i, p := range sessions {
		v := SessionView{PrepSession: p, Dishes: bySession[p.ID]}
		if v.Dishes == nil {
			v.Dishes = []DishView{}
		}
		for _, d := range v.Dishes {
			v.Remaining += d.Remaining
		}
		out[i] = v
	}
	return out, nil
}

func (s *Service) Sessions(ctx context.Context, hh string) ([]SessionView, error) {
	ps, err := s.repo.PrepSessions(ctx, hh)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, hh, ps)
}

func (s *Service) Session(ctx context.Context, hh, id string) (SessionView, error) {
	p, err := s.repo.PrepSessionByID(ctx, hh, id)
	if err != nil {
		return SessionView{}, err
	}
	vs, err := s.views(ctx, hh, []domain.PrepSession{p})
	if err != nil {
		return SessionView{}, err
	}
	return vs[0], nil
}

func (s *Service) CreateSession(ctx context.Context, hh string, in SessionInput) (SessionView, error) {
	name, week, err := s.checkSession(in)
	if err != nil {
		return SessionView{}, err
	}
	p := domain.PrepSession{ID: auth.NewID(), HouseholdID: hh, Date: in.Date, Name: name, Week: week, CreatedAt: s.now()}
	if err := s.repo.InsertPrepSession(ctx, p); err != nil {
		return SessionView{}, err
	}
	return s.Session(ctx, hh, p.ID)
}

// syncPantry makes stock match the saved session's Cooked flag; the hook is idempotent, so saving
// again after a failure converges.
func (s *Service) syncPantry(ctx context.Context, p domain.PrepSession) error {
	if s.hook == nil {
		return nil
	}
	var err error
	if p.Cooked {
		var dishes []domain.PrepDish
		if dishes, err = s.repo.PrepDishes(ctx, p.HouseholdID, []string{p.ID}); err == nil {
			err = s.hook.SessionCooked(ctx, p.HouseholdID, p, dishes)
		}
	} else {
		err = s.hook.SessionUncooked(ctx, p.HouseholdID, p.ID)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPantry, err)
	}
	return nil
}

// recook brings stock in line with a cooked session's current dishes after one was added, resized
// or removed: it puts back what was taken and takes it again for the dishes as they are now. Both
// steps are atomic and idempotent, so after a failure (ErrPantry) saving the session again converges.
func (s *Service) recook(ctx context.Context, hh, sessionID string) error {
	if s.hook == nil {
		return nil
	}
	p, err := s.repo.PrepSessionByID(ctx, hh, sessionID)
	if err != nil || !p.Cooked {
		return err
	}
	if err := s.hook.SessionUncooked(ctx, hh, sessionID); err != nil {
		return fmt.Errorf("%w: %w", ErrPantry, err)
	}
	return s.syncPantry(ctx, p)
}

// UpdateSession saves the session, then syncs the pantry with its Cooked flag. A pantry failure is
// reported as ErrPantry with the new state kept.
func (s *Service) UpdateSession(ctx context.Context, hh, id string, in SessionInput) (SessionView, error) {
	old, err := s.repo.PrepSessionByID(ctx, hh, id)
	if err != nil {
		return SessionView{}, err
	}
	name, week, err := s.checkSession(in)
	if err != nil {
		return SessionView{}, err
	}
	p := old
	p.Date, p.Name, p.Week, p.Cooked = in.Date, name, week, in.Cooked
	if err := s.repo.UpdatePrepSession(ctx, p); err != nil {
		return SessionView{}, err
	}
	if err := s.syncPantry(ctx, p); err != nil {
		return SessionView{}, err
	}
	return s.Session(ctx, hh, id)
}

// DeleteSession refuses (store.ErrInUse) while any of its dishes is planned in meals. A cooked
// session first puts its ingredients back into the pantry; if that fails nothing is deleted.
func (s *Service) DeleteSession(ctx context.Context, hh, id string) error {
	p, err := s.repo.PrepSessionByID(ctx, hh, id)
	if err != nil {
		return err
	}
	dishes, err := s.repo.PrepDishes(ctx, hh, []string{id})
	if err != nil {
		return err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return err
	}
	for _, d := range dishes {
		if hasDish(allocs, d.ID) {
			return store.ErrInUse
		}
	}
	if p.Cooked && s.hook != nil {
		if err := s.hook.SessionUncooked(ctx, hh, id); err != nil {
			return fmt.Errorf("%w: %w", ErrPantry, err)
		}
	}
	return s.repo.DeletePrepSession(ctx, hh, id)
}

func hasDish(allocs []domain.PlanEntry, dishID string) bool {
	for _, e := range allocs {
		if e.PrepDishID == dishID {
			return true
		}
	}
	return false
}

func hasFreezer(allocs []domain.PlanEntry, itemID string) bool {
	for _, e := range allocs {
		if e.FreezerItemID == itemID {
			return true
		}
	}
	return false
}

func (s *Service) dishView(ctx context.Context, hh string, d domain.PrepDish) (DishView, error) {
	sv, err := s.Session(ctx, hh, d.SessionID)
	if err != nil {
		return DishView{}, err
	}
	for _, v := range sv.Dishes {
		if v.ID == d.ID {
			return v, nil
		}
	}
	return DishView{}, store.ErrNotFound
}

func (s *Service) AddDish(ctx context.Context, hh, sessionID, recipeID string, portions int) (DishView, error) {
	if err := checkPortions(portions); err != nil {
		return DishView{}, err
	}
	if recipeID == "" {
		return DishView{}, invalid("recipe is required")
	}
	if _, err := s.repo.PrepSessionByID(ctx, hh, sessionID); err != nil {
		return DishView{}, err
	}
	if _, err := s.recipes.RecipeByID(ctx, hh, recipeID); err != nil {
		return DishView{}, err
	}
	d := domain.PrepDish{ID: auth.NewID(), HouseholdID: hh, SessionID: sessionID, RecipeID: recipeID, Portions: portions}
	if err := s.repo.InsertPrepDish(ctx, d); err != nil {
		return DishView{}, err
	}
	d, err := s.repo.PrepDishByID(ctx, hh, d.ID)
	if err != nil {
		return DishView{}, err
	}
	if err := s.recook(ctx, hh, sessionID); err != nil {
		return DishView{}, err
	}
	return s.dishView(ctx, hh, d)
}

// UpdateDish changes a dish's portions and written-off count; both must leave Remaining >= 0.
func (s *Service) UpdateDish(ctx context.Context, hh, id string, portions, writtenOff int) (DishView, error) {
	d, err := s.repo.PrepDishByID(ctx, hh, id)
	if err != nil {
		return DishView{}, err
	}
	if err := checkPortions(portions); err != nil {
		return DishView{}, err
	}
	items, err := s.repo.FreezerItems(ctx, hh)
	if err != nil {
		return DishView{}, err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return DishView{}, err
	}
	used := DishAllocated(id, allocs) + DishFrozen(id, items)
	if writtenOff < 0 {
		return DishView{}, invalid("written off must not be negative")
	}
	if portions < used+writtenOff {
		return DishView{}, invalid("portions cannot be fewer than %d already planned, frozen or written off", used+writtenOff)
	}
	resized := d.Portions != portions
	d.Portions, d.WrittenOff = portions, writtenOff
	if err := s.repo.UpdatePrepDish(ctx, d); err != nil {
		return DishView{}, err
	}
	if resized {
		if err := s.recook(ctx, hh, d.SessionID); err != nil {
			return DishView{}, err
		}
	}
	return s.dishView(ctx, hh, d)
}

// DeleteDish refuses (store.ErrInUse) while meals are planned from it; frozen items stay. In a
// cooked session the pantry is recounted without the dish.
func (s *Service) DeleteDish(ctx context.Context, hh, id string) error {
	d, err := s.repo.PrepDishByID(ctx, hh, id)
	if err != nil {
		return err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return err
	}
	if hasDish(allocs, id) {
		return store.ErrInUse
	}
	if err := s.repo.DeletePrepDish(ctx, hh, id); err != nil {
		return err
	}
	return s.recook(ctx, hh, d.SessionID)
}

func (s *Service) freezerView(ctx context.Context, hh string, f domain.FreezerItem, allocs []domain.PlanEntry, titles map[string]string) (FreezerView, error) {
	t, err := s.title(ctx, hh, f.RecipeID, titles)
	if err != nil {
		return FreezerView{}, err
	}
	v := FreezerView{FreezerItem: f, Title: t, Allocated: FreezerAllocated(f.ID, allocs), Remaining: FreezerRemaining(f, allocs), Allocations: []domain.PlanEntry{}}
	if v.Title == "" {
		v.Title = f.Label
	}
	for _, e := range allocs {
		if e.FreezerItemID == f.ID {
			v.Allocations = append(v.Allocations, e)
		}
	}
	return v, nil
}

func (s *Service) oneFreezer(ctx context.Context, hh, id string) (FreezerView, error) {
	f, err := s.repo.FreezerItemByID(ctx, hh, id)
	if err != nil {
		return FreezerView{}, err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return FreezerView{}, err
	}
	return s.freezerView(ctx, hh, f, allocs, map[string]string{})
}

// Freeze moves portions of a dish into the freezer, labelled with the recipe title and dated today.
// frozenOn is optional (YYYY-MM-DD); empty means today by the server clock.
func (s *Service) Freeze(ctx context.Context, hh, dishID string, portions int, frozenOn string) (FreezerView, error) {
	if frozenOn == "" {
		frozenOn = s.now().Format(dateLayout)
	} else if err := validDate(frozenOn); err != nil {
		return FreezerView{}, err
	}
	d, err := s.repo.PrepDishByID(ctx, hh, dishID)
	if err != nil {
		return FreezerView{}, err
	}
	items, err := s.repo.FreezerItems(ctx, hh)
	if err != nil {
		return FreezerView{}, err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return FreezerView{}, err
	}
	if rem := DishRemaining(d, items, allocs); portions < 1 || portions > rem {
		return FreezerView{}, invalid("portions must be 1-%d", max(rem, 0))
	}
	r, err := s.recipes.RecipeByID(ctx, hh, d.RecipeID)
	if err != nil {
		return FreezerView{}, err
	}
	f := domain.FreezerItem{ID: auth.NewID(), HouseholdID: hh, DishID: d.ID, RecipeID: d.RecipeID,
		Label: r.Title, Portions: portions, FrozenOn: frozenOn}
	if err := s.repo.InsertFreezerItem(ctx, f); err != nil {
		return FreezerView{}, err
	}
	return s.oneFreezer(ctx, hh, f.ID)
}

// Freezer lists items with portions still unplanned.
func (s *Service) Freezer(ctx context.Context, hh string) ([]FreezerView, error) {
	items, err := s.repo.FreezerItems(ctx, hh)
	if err != nil {
		return nil, err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	out := []FreezerView{}
	for _, f := range items {
		if FreezerRemaining(f, allocs) <= 0 {
			continue
		}
		v, err := s.freezerView(ctx, hh, f, allocs, titles)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// freezerFields validates the input and fills the label and date defaults.
func (s *Service) freezerFields(ctx context.Context, hh string, in FreezerInput) (domain.FreezerItem, error) {
	if err := checkPortions(in.Portions); err != nil {
		return domain.FreezerItem{}, err
	}
	label := strings.TrimSpace(in.Label)
	if utf8.RuneCountInString(label) > maxLabelLen || domain.HasControl(label) {
		return domain.FreezerItem{}, invalid("label must be at most %d characters without control characters", maxLabelLen)
	}
	if label == "" && in.RecipeID == "" {
		return domain.FreezerItem{}, invalid("recipe or label is required")
	}
	if in.RecipeID != "" {
		r, err := s.recipes.RecipeByID(ctx, hh, in.RecipeID)
		if err != nil {
			return domain.FreezerItem{}, err
		}
		if label == "" {
			label = r.Title
		}
	}
	on := in.FrozenOn
	if on == "" {
		on = s.now().Format(dateLayout)
	} else if err := validDate(on); err != nil {
		return domain.FreezerItem{}, err
	}
	return domain.FreezerItem{HouseholdID: hh, RecipeID: in.RecipeID, Label: label, Portions: in.Portions, FrozenOn: on}, nil
}

func (s *Service) AddFreezer(ctx context.Context, hh string, in FreezerInput) (FreezerView, error) {
	f, err := s.freezerFields(ctx, hh, in)
	if err != nil {
		return FreezerView{}, err
	}
	f.ID = auth.NewID()
	if err := s.repo.InsertFreezerItem(ctx, f); err != nil {
		return FreezerView{}, err
	}
	return s.oneFreezer(ctx, hh, f.ID)
}

// UpdateFreezer rewrites the item (its link to a dish is kept); portions may not drop below the
// planned ones.
func (s *Service) UpdateFreezer(ctx context.Context, hh, id string, in FreezerInput) (FreezerView, error) {
	old, err := s.repo.FreezerItemByID(ctx, hh, id)
	if err != nil {
		return FreezerView{}, err
	}
	if in.RecipeID == "" {
		in.RecipeID = old.RecipeID // an update without a recipe keeps the stored one
	}
	f, err := s.freezerFields(ctx, hh, in)
	if err != nil {
		return FreezerView{}, err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return FreezerView{}, err
	}
	if inc := f.Portions - old.Portions; old.DishID != "" && inc > 0 {
		d, err := s.repo.PrepDishByID(ctx, hh, old.DishID)
		if err != nil {
			return FreezerView{}, err
		}
		items, err := s.repo.FreezerItems(ctx, hh)
		if err != nil {
			return FreezerView{}, err
		}
		if rem := DishRemaining(d, items, allocs); inc > rem {
			return FreezerView{}, invalid("portions can grow by at most %d, what is left of the dish", max(rem, 0))
		}
	}
	if used := FreezerAllocated(id, allocs); f.Portions < used {
		return FreezerView{}, invalid("portions cannot be fewer than the %d already planned", used)
	}
	f.ID, f.DishID = id, old.DishID
	if err := s.repo.UpdateFreezerItem(ctx, f); err != nil {
		return FreezerView{}, err
	}
	return s.oneFreezer(ctx, hh, id)
}

// DeleteFreezer refuses (store.ErrInUse) while meals are planned from it.
func (s *Service) DeleteFreezer(ctx context.Context, hh, id string) error {
	if _, err := s.repo.FreezerItemByID(ctx, hh, id); err != nil {
		return err
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return err
	}
	if hasFreezer(allocs, id) {
		return store.ErrInUse
	}
	return s.repo.DeleteFreezerItem(ctx, hh, id)
}

// Available reports the recipe, label and portions left of a dish or freezer item (exactly one id).
// The servings of exceptEntryID, an entry being edited, count as available.
func (s *Service) Available(ctx context.Context, hh, dishID, freezerItemID, exceptEntryID string) (recipeID, label string, remaining int, err error) {
	if (dishID == "") == (freezerItemID == "") {
		return "", "", 0, invalid("exactly one of dish and freezer item is required")
	}
	allocs, err := s.repo.Allocations(ctx, hh)
	if err != nil {
		return "", "", 0, err
	}
	rest := allocs[:0:0]
	for _, e := range allocs {
		if e.ID == exceptEntryID && exceptEntryID != "" {
			continue
		}
		rest = append(rest, e)
	}
	if dishID != "" {
		d, err := s.repo.PrepDishByID(ctx, hh, dishID)
		if err != nil {
			return "", "", 0, err
		}
		items, err := s.repo.FreezerItems(ctx, hh)
		if err != nil {
			return "", "", 0, err
		}
		t, err := s.title(ctx, hh, d.RecipeID, map[string]string{})
		if err != nil {
			return "", "", 0, err
		}
		return d.RecipeID, t, DishRemaining(d, items, rest), nil
	}
	f, err := s.repo.FreezerItemByID(ctx, hh, freezerItemID)
	if err != nil {
		return "", "", 0, err
	}
	return f.RecipeID, f.Label, FreezerRemaining(f, rest), nil
}

// PortionsPerMeal is the household's default portions per meal.
func (s *Service) PortionsPerMeal(ctx context.Context, hh string) (int, error) {
	return s.repo.PortionsPerMeal(ctx, hh)
}

func (s *Service) SetPortionsPerMeal(ctx context.Context, hh string, n int) error {
	if n < 1 || n > maxPerMeal {
		return invalid("portions per meal must be 1-%d", maxPerMeal)
	}
	return s.repo.SetPortionsPerMeal(ctx, hh, n)
}
