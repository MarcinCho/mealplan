package httpapi

import (
	"net/http"

	"mealplan/internal/domain"
	"mealplan/internal/product"
)

type macrosJSON struct {
	Kcal    *float64 `json:"kcal"`
	Protein *float64 `json:"protein"`
	Carbs   *float64 `json:"carbs"`
	Fat     *float64 `json:"fat"`
}

type productJSON struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Per100g       macrosJSON `json:"per100g"`
	PieceGrams    *float64   `json:"pieceGrams"`
	DensityGPerML *float64   `json:"densityGPerMl"`
	Aliases       []string   `json:"aliases"`
}

type productInJSON struct {
	Name          string     `json:"name"`
	Per100g       macrosJSON `json:"per100g"`
	PieceGrams    *float64   `json:"pieceGrams"`
	DensityGPerML *float64   `json:"densityGPerMl"`
	Aliases       []string   `json:"aliases"`
}

func toProductJSON(p domain.Product) productJSON {
	return productJSON{ID: p.ID, Name: p.Name, Per100g: macrosJSON(p.Per100g), PieceGrams: p.PieceGrams,
		DensityGPerML: p.DensityGPerML, Aliases: orEmpty(p.Aliases)}
}

func (in productInJSON) toInput() product.Input {
	return product.Input{Name: in.Name, Per100g: domain.Macros(in.Per100g), PieceGrams: in.PieceGrams, DensityGPerML: in.DensityGPerML, Aliases: in.Aliases}
}

func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.Products.List(r.Context(), principalFrom(r).HouseholdID, r.URL.Query().Get("q"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]productJSON, 0, len(list))
	for _, p := range list {
		out = append(out, toProductJSON(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var in productInJSON
	if !decode(w, r, &in) {
		return
	}
	p, err := s.deps.Products.Create(r.Context(), principalFrom(r).HouseholdID, in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProductJSON(p))
}

func (s *Server) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	var in productInJSON
	if !decode(w, r, &in) {
		return
	}
	p, err := s.deps.Products.Update(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toProductJSON(p))
}

func (s *Server) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Products.Delete(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
