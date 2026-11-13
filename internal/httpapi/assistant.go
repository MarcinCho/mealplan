package httpapi

import (
	"net/http"
	"time"

	"mealplan/internal/assistant"
	"mealplan/internal/auth"
	"mealplan/internal/domain"
)

// modelCallDeadline lets a handler that calls the model outlive the server's write timeout.
const modelCallDeadline = 75 * time.Second

// extendWriteDeadline gives the model call time to finish; writers that cannot extend it are left alone.
func extendWriteDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(modelCallDeadline))
}

// allowModelCall applies the per-user import limit to handlers that call the model.
func (s *Server) allowModelCall(w http.ResponseWriter, r *http.Request) bool {
	if ok, retry := s.importLimit.allow(principalFrom(r).UserID); !ok {
		s.fail(w, r, &auth.RateLimitedError{RetryAfter: retry})
		return false
	}
	return true
}

type assistantStatusJSON struct {
	Configured   bool   `json:"configured"`
	KeyHint      string `json:"keyHint"`
	Model        string `json:"model"`
	DefaultModel string `json:"defaultModel"`
}

func (s *Server) writeAssistantStatus(w http.ResponseWriter, r *http.Request) {
	configured, hint, model, err := s.deps.Assistant.Status(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, assistantStatusJSON{Configured: configured, KeyHint: hint, Model: model, DefaultModel: assistant.DefaultModel})
}

func (s *Server) handleGetAssistant(w http.ResponseWriter, r *http.Request) {
	s.writeAssistantStatus(w, r)
}

type assistantSettingsInJSON struct {
	Key   *string `json:"key"`
	Model string  `json:"model"`
}

func (s *Server) handlePutAssistant(w http.ResponseWriter, r *http.Request) {
	var in assistantSettingsInJSON
	if !decode(w, r, &in) {
		return
	}
	if err := s.deps.Assistant.Configure(r.Context(), principalFrom(r), in.Key, in.Model); err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeAssistantStatus(w, r)
}

func (s *Server) handleDeleteAssistantKey(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	_, _, model, err := s.deps.Assistant.Status(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	empty := ""
	if err := s.deps.Assistant.Configure(r.Context(), p, &empty, model); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestAssistant(w http.ResponseWriter, r *http.Request) {
	if !s.allowModelCall(w, r) {
		return
	}
	extendWriteDeadline(w)
	if err := s.deps.Assistant.Test(r.Context(), principalFrom(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ingredientQueryJSON struct {
	Name     string   `json:"name"`
	Quantity *float64 `json:"quantity"`
	Unit     string   `json:"unit"`
}

type matchInJSON struct {
	Ingredients []ingredientQueryJSON `json:"ingredients"`
}

type newProductJSON struct {
	Name          string     `json:"name"`
	Per100g       macrosJSON `json:"per100g"`
	PieceGrams    *float64   `json:"pieceGrams"`
	DensityGPerML *float64   `json:"densityGPerMl"`
	Aliases       []string   `json:"aliases"`
}

type suggestionJSON struct {
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	ProductID   string          `json:"productId,omitempty"`
	ProductName string          `json:"productName,omitempty"`
	NewProduct  *newProductJSON `json:"newProduct,omitempty"`
	Reason      string          `json:"reason"`
}

func (s *Server) handleAssistantIngredients(w http.ResponseWriter, r *http.Request) {
	var in matchInJSON
	if !decode(w, r, &in) {
		return
	}
	if !s.allowModelCall(w, r) {
		return
	}
	extendWriteDeadline(w)
	qs := make([]assistant.IngredientQuery, 0, len(in.Ingredients))
	for _, i := range in.Ingredients {
		qs = append(qs, assistant.IngredientQuery{Name: i.Name, Quantity: i.Quantity, Unit: i.Unit})
	}
	got, err := s.deps.Assistant.MatchIngredients(r.Context(), principalFrom(r), qs)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]suggestionJSON, 0, len(got))
	for _, g := range got {
		sj := suggestionJSON{Name: g.Name, Kind: g.Kind, ProductID: g.ProductID, ProductName: g.ProductName, Reason: g.Reason}
		if g.New != nil {
			sj.NewProduct = &newProductJSON{Name: g.New.Name, Per100g: macrosJSON(domain.Macros(g.New.Per100g)),
				PieceGrams: g.New.PieceGrams, DensityGPerML: g.New.DensityGPerML, Aliases: orEmpty(g.New.Aliases)}
		}
		out = append(out, sj)
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

type candidateJSON struct {
	Name             string   `json:"name"`
	NormalizedName   string   `json:"normalizedName"`
	CurrentProductID string   `json:"currentProductId"`
	RecipeIDs        []string `json:"recipeIds"`
	RecipeTitles     []string `json:"recipeTitles"`
}

func (s *Server) handleIngredientCandidates(w http.ResponseWriter, r *http.Request) {
	got, err := s.deps.Assistant.IngredientCandidates(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]candidateJSON, 0, len(got))
	for _, c := range got {
		out = append(out, candidateJSON{Name: c.Name, NormalizedName: c.NormalizedName, CurrentProductID: c.CurrentProductID,
			RecipeIDs: orEmpty(c.RecipeIDs), RecipeTitles: orEmpty(c.RecipeTitles)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": out})
}

func (s *Server) handleProductBatches(w http.ResponseWriter, r *http.Request) {
	got, err := s.deps.Assistant.ProductBatches(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([][]string, 0, len(got))
	for _, b := range got {
		out = append(out, orEmpty(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": out})
}

type duplicatesInJSON struct {
	ProductIDs []string `json:"productIds"`
}

type duplicateGroupJSON struct {
	KeepID    string   `json:"keepId"`
	AbsorbIDs []string `json:"absorbIds"`
	Reason    string   `json:"reason"`
}

func (s *Server) handleAssistantDuplicates(w http.ResponseWriter, r *http.Request) {
	var in duplicatesInJSON
	if !decode(w, r, &in) {
		return
	}
	if !s.allowModelCall(w, r) {
		return
	}
	extendWriteDeadline(w)
	got, err := s.deps.Assistant.FindDuplicates(r.Context(), principalFrom(r), in.ProductIDs)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]duplicateGroupJSON, 0, len(got))
	for _, g := range got {
		out = append(out, duplicateGroupJSON{KeepID: g.KeepID, AbsorbIDs: orEmpty(g.AbsorbIDs), Reason: g.Reason})
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}
