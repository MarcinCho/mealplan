package httpapi

import (
	"net/http"

	"mealplan/internal/domain"
	"mealplan/internal/plan"
)

type entryJSON struct {
	ID         string  `json:"id"`
	Date       string  `json:"date"`
	Slot       string  `json:"slot"`
	RecipeID   *string `json:"recipeId"`
	FreeText   string  `json:"freeText"`
	Servings   int     `json:"servings"`
	CookUserID *string `json:"cookUserId"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toEntryJSON(e domain.PlanEntry) entryJSON {
	return entryJSON{ID: e.ID, Date: e.Date, Slot: string(e.Slot), RecipeID: strPtr(e.RecipeID),
		FreeText: e.FreeText, Servings: e.Servings, CookUserID: strPtr(e.CookUserID)}
}

type entryInJSON struct {
	Date       string  `json:"date"`
	Slot       string  `json:"slot"`
	RecipeID   *string `json:"recipeId"`
	FreeText   string  `json:"freeText"`
	Servings   int     `json:"servings"`
	CookUserID *string `json:"cookUserId"`
}

func (in entryInJSON) toInput() plan.EntryInput {
	out := plan.EntryInput{Date: in.Date, Slot: domain.Slot(in.Slot), FreeText: in.FreeText, Servings: in.Servings}
	if in.RecipeID != nil {
		out.RecipeID = *in.RecipeID
	}
	if in.CookUserID != nil {
		out.CookUserID = *in.CookUserID
	}
	return out
}

func (s *Server) handleGetWeek(w http.ResponseWriter, r *http.Request) {
	week := r.PathValue("week")
	entries, err := s.deps.Plan.Week(r.Context(), principalFrom(r).HouseholdID, week)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, toEntryJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"week": week, "entries": out})
}

func (s *Server) handleAddEntry(w http.ResponseWriter, r *http.Request) {
	var in entryInJSON
	if !decode(w, r, &in) {
		return
	}
	e, err := s.deps.Plan.Add(r.Context(), principalFrom(r).HouseholdID, r.PathValue("week"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEntryJSON(e))
}

func (s *Server) handleEditEntry(w http.ResponseWriter, r *http.Request) {
	var in entryInJSON
	if !decode(w, r, &in) {
		return
	}
	e, err := s.deps.Plan.Edit(r.Context(), principalFrom(r).HouseholdID, r.PathValue("week"), r.PathValue("id"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEntryJSON(e))
}

func (s *Server) handleRemoveEntry(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Plan.Remove(r.Context(), principalFrom(r).HouseholdID, r.PathValue("week"), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCopyWeek(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FromWeek string `json:"fromWeek"`
	}
	if !decode(w, r, &in) {
		return
	}
	n, err := s.deps.Plan.Copy(r.Context(), principalFrom(r).HouseholdID, in.FromWeek, r.PathValue("week"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"copied": n})
}
