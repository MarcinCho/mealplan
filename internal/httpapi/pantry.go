package httpapi

import (
	"net/http"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/pantry"
)

type pantryItemJSON struct {
	ID        string   `json:"id"`
	ProductID string   `json:"productId"`
	Name      string   `json:"name"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
	ExpiresOn string   `json:"expiresOn"`
	Staple    bool     `json:"staple"`
}

type pantryInJSON struct {
	ProductID string   `json:"productId"`
	Name      string   `json:"name"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
	ExpiresOn string   `json:"expiresOn"`
	Staple    bool     `json:"staple"`
}

func toPantryJSON(it domain.PantryItem) pantryItemJSON {
	return pantryItemJSON{ID: it.ID, ProductID: it.ProductID, Name: it.Name, Quantity: it.Quantity, Unit: it.Unit, ExpiresOn: it.ExpiresOn, Staple: it.Staple}
}

func (in pantryInJSON) toInput() pantry.Input {
	return pantry.Input{ProductID: in.ProductID, Name: in.Name, Quantity: in.Quantity, Unit: in.Unit, ExpiresOn: in.ExpiresOn, Staple: in.Staple}
}

func formatReviewed(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func (s *Server) handleListPantry(w http.ResponseWriter, r *http.Request) {
	items, reviewed, err := s.deps.Pantry.List(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]pantryItemJSON, 0, len(items))
	for _, it := range items {
		out = append(out, toPantryJSON(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "reviewedAt": formatReviewed(reviewed)})
}

func (s *Server) handleAddPantry(w http.ResponseWriter, r *http.Request) {
	var in pantryInJSON
	if !decode(w, r, &in) {
		return
	}
	it, err := s.deps.Pantry.Add(r.Context(), principalFrom(r).HouseholdID, in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPantryJSON(it))
}

func (s *Server) handleUpdatePantry(w http.ResponseWriter, r *http.Request) {
	var in pantryInJSON
	if !decode(w, r, &in) {
		return
	}
	it, err := s.deps.Pantry.Update(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPantryJSON(it))
}

func (s *Server) handleDeletePantry(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Pantry.Delete(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReviewPantry(w http.ResponseWriter, r *http.Request) {
	t, err := s.deps.Pantry.MarkReviewed(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviewedAt": t.UTC().Format(time.RFC3339)})
}

func (s *Server) handleMoveToPantry(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Week string `json:"week"`
	}
	if !decode(w, r, &in) {
		return
	}
	n, err := s.deps.Shopping.MoveCheckedToPantry(r.Context(), principalFrom(r).HouseholdID, in.Week)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"moved": n})
}
