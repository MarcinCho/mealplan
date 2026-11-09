package httpapi

import (
	"net/http"

	"mealplan/internal/domain"
	"mealplan/internal/mealprep"
)

type allocationJSON struct {
	EntryID  string `json:"entryId"`
	Date     string `json:"date"`
	Slot     string `json:"slot"`
	Servings int    `json:"servings"`
}

type dishJSON struct {
	ID          string           `json:"id"`
	RecipeID    string           `json:"recipeId"`
	RecipeTitle string           `json:"recipeTitle"`
	Portions    int              `json:"portions"`
	WrittenOff  int              `json:"writtenOff"`
	Allocated   int              `json:"allocated"`
	Frozen      int              `json:"frozen"`
	Remaining   int              `json:"remaining"`
	Allocations []allocationJSON `json:"allocations"`
}

type sessionJSON struct {
	ID        string     `json:"id"`
	Date      string     `json:"date"`
	Name      string     `json:"name"`
	Week      string     `json:"week"`
	Cooked    bool       `json:"cooked"`
	Remaining int        `json:"remaining"`
	Dishes    []dishJSON `json:"dishes"`
}

type freezerJSON struct {
	ID          string           `json:"id"`
	DishID      *string          `json:"dishId"`
	RecipeID    *string          `json:"recipeId"`
	Title       string           `json:"title"`
	Portions    int              `json:"portions"`
	Allocated   int              `json:"allocated"`
	Remaining   int              `json:"remaining"`
	FrozenOn    string           `json:"frozenOn"`
	Allocations []allocationJSON `json:"allocations"`
}

func toAllocations(es []domain.PlanEntry) []allocationJSON {
	out := make([]allocationJSON, 0, len(es))
	for _, e := range es {
		out = append(out, allocationJSON{EntryID: e.ID, Date: e.Date, Slot: string(e.Slot), Servings: e.Servings})
	}
	return out
}

func toDishJSON(d mealprep.DishView) dishJSON {
	return dishJSON{ID: d.ID, RecipeID: d.RecipeID, RecipeTitle: d.RecipeTitle, Portions: d.Portions, WrittenOff: d.WrittenOff,
		Allocated: d.Allocated, Frozen: d.Frozen, Remaining: d.Remaining, Allocations: toAllocations(d.Allocations)}
}

func toSessionJSON(s mealprep.SessionView) sessionJSON {
	dishes := make([]dishJSON, 0, len(s.Dishes))
	for _, d := range s.Dishes {
		dishes = append(dishes, toDishJSON(d))
	}
	return sessionJSON{ID: s.ID, Date: s.Date, Name: s.Name, Week: s.Week, Cooked: s.Cooked, Remaining: s.Remaining, Dishes: dishes}
}

func toFreezerJSON(f mealprep.FreezerView) freezerJSON {
	return freezerJSON{ID: f.ID, DishID: strPtr(f.DishID), RecipeID: strPtr(f.RecipeID), Title: f.Title, Portions: f.Portions,
		Allocated: f.Allocated, Remaining: f.Remaining, FrozenOn: f.FrozenOn, Allocations: toAllocations(f.Allocations)}
}

type sessionInJSON struct {
	Date   string `json:"date"`
	Name   string `json:"name"`
	Week   string `json:"week"`
	Cooked bool   `json:"cooked"`
}

func (in sessionInJSON) toInput() mealprep.SessionInput {
	return mealprep.SessionInput{Date: in.Date, Name: in.Name, Week: in.Week, Cooked: in.Cooked}
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.MealPrep.Sessions(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]sessionJSON, 0, len(list))
	for _, v := range list {
		out = append(out, toSessionJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var in sessionInJSON
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.CreateSession(r.Context(), principalFrom(r).HouseholdID, in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSessionJSON(v))
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	var in sessionInJSON
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.UpdateSession(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSessionJSON(v))
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.MealPrep.DeleteSession(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddDish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RecipeID string `json:"recipeId"`
		Portions int    `json:"portions"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.AddDish(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.RecipeID, in.Portions)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDishJSON(v))
}

func (s *Server) handleUpdateDish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Portions   int `json:"portions"`
		WrittenOff int `json:"writtenOff"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.UpdateDish(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.Portions, in.WrittenOff)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDishJSON(v))
}

func (s *Server) handleDeleteDish(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.MealPrep.DeleteDish(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFreezeDish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Portions int    `json:"portions"`
		FrozenOn string `json:"frozenOn"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.Freeze(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.Portions, in.FrozenOn)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFreezerJSON(v))
}

type freezerInJSON struct {
	RecipeID *string `json:"recipeId"`
	Label    string  `json:"label"`
	Portions int     `json:"portions"`
	FrozenOn string  `json:"frozenOn"`
}

func (in freezerInJSON) toInput() mealprep.FreezerInput {
	out := mealprep.FreezerInput{Label: in.Label, Portions: in.Portions, FrozenOn: in.FrozenOn}
	if in.RecipeID != nil {
		out.RecipeID = *in.RecipeID
	}
	return out
}

func (s *Server) handleListFreezer(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.MealPrep.Freezer(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]freezerJSON, 0, len(list))
	for _, v := range list {
		out = append(out, toFreezerJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleAddFreezer(w http.ResponseWriter, r *http.Request) {
	var in freezerInJSON
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.AddFreezer(r.Context(), principalFrom(r).HouseholdID, in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFreezerJSON(v))
}

func (s *Server) handleUpdateFreezer(w http.ResponseWriter, r *http.Request) {
	var in freezerInJSON
	if !decode(w, r, &in) {
		return
	}
	v, err := s.deps.MealPrep.UpdateFreezer(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toFreezerJSON(v))
}

func (s *Server) handleDeleteFreezer(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.MealPrep.DeleteFreezer(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetHouseholdSettings(w http.ResponseWriter, r *http.Request) {
	n, err := s.deps.MealPrep.PortionsPerMeal(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"portionsPerMeal": n})
}

func (s *Server) handlePutHouseholdSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PortionsPerMeal int `json:"portionsPerMeal"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.deps.MealPrep.SetPortionsPerMeal(r.Context(), principalFrom(r).HouseholdID, in.PortionsPerMeal); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"portionsPerMeal": in.PortionsPerMeal})
}
