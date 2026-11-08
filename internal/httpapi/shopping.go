package httpapi

import (
	"fmt"
	"net/http"

	"mealplan/internal/domain"
	"mealplan/internal/shopping"
)

type itemJSON struct {
	ID       string   `json:"id"`
	Week     string   `json:"week"`
	Name     string   `json:"name"`
	Quantity *float64 `json:"quantity"`
	Unit     string   `json:"unit"`
	Checked  bool     `json:"checked"`
	Manual   bool     `json:"manual"`

	HaveQuantity *float64 `json:"haveQuantity"`
	Moved        bool     `json:"moved"`
}

func toItemJSON(it domain.ShoppingItem) itemJSON {
	return itemJSON{ID: it.ID, Week: it.Week, Name: it.Name, Quantity: it.Quantity, Unit: it.Unit, Checked: it.Checked, Manual: it.Manual, HaveQuantity: it.HaveQuantity, Moved: it.Moved}
}

func writeList(w http.ResponseWriter, week string, items []domain.ShoppingItem) {
	out := make([]itemJSON, 0, len(items))
	for _, it := range items {
		out = append(out, toItemJSON(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"week": week, "items": out})
}

func (s *Server) handleListShopping(w http.ResponseWriter, r *http.Request) {
	week := r.URL.Query().Get("week")
	items, err := s.deps.Shopping.List(r.Context(), principalFrom(r).HouseholdID, week)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeList(w, week, items)
}

func (s *Server) handleGenerateShopping(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Week string `json:"week"`
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !decode(w, r, &in) {
		return
	}
	items, err := s.deps.Shopping.Generate(r.Context(), principalFrom(r).HouseholdID, in.Week, in.From, in.To)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeList(w, in.Week, items)
}

func (s *Server) handleAddShoppingItem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Week     string   `json:"week"`
		Name     string   `json:"name"`
		Quantity *float64 `json:"quantity"`
		Unit     string   `json:"unit"`
	}
	if !decode(w, r, &in) {
		return
	}
	it, err := s.deps.Shopping.AddManual(r.Context(), principalFrom(r).HouseholdID, in.Week,
		shopping.ItemInput{Name: in.Name, Quantity: in.Quantity, Unit: in.Unit})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemJSON(it))
}

func (s *Server) handlePatchShoppingItem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Checked  *bool    `json:"checked"`
		Name     *string  `json:"name"`
		Quantity *float64 `json:"quantity"`
		Unit     string   `json:"unit"`
	}
	if !decode(w, r, &in) {
		return
	}
	hid, id := principalFrom(r).HouseholdID, r.PathValue("id")
	if in.Checked == nil && in.Name == nil {
		s.fail(w, r, fmt.Errorf("%w: provide checked or name", domain.ErrInvalid))
		return
	}
	if in.Name != nil {
		if _, err := s.deps.Shopping.Edit(r.Context(), hid, id, shopping.ItemInput{Name: *in.Name, Quantity: in.Quantity, Unit: in.Unit}); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	if in.Checked != nil {
		if err := s.deps.Shopping.SetChecked(r.Context(), hid, id, *in.Checked); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteShoppingItem(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Shopping.Remove(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
