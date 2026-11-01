package httpapi

import (
	"net/http"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/recipe"
)

type ingredientJSON struct {
	Name     string   `json:"name"`
	Quantity *float64 `json:"quantity"`
	Unit     string   `json:"unit"`
}

type recipeJSON struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	BaseServings int              `json:"baseServings"`
	SourceURL    string           `json:"sourceUrl"`
	Tags         []string         `json:"tags"`
	Ingredients  []ingredientJSON `json:"ingredients"`
	Steps        []string         `json:"steps"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type recipeInJSON struct {
	Title        string           `json:"title"`
	BaseServings int              `json:"baseServings"`
	SourceURL    string           `json:"sourceUrl"`
	Tags         []string         `json:"tags"`
	Ingredients  []ingredientJSON `json:"ingredients"`
	Steps        []string         `json:"steps"`
}

func toRecipeJSON(r domain.Recipe) recipeJSON {
	out := recipeJSON{ID: r.ID, Title: r.Title, BaseServings: r.BaseServings, SourceURL: r.SourceURL,
		Tags: orEmpty(r.Tags), Steps: orEmpty(r.Steps), Ingredients: []ingredientJSON{},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	for _, i := range r.Ingredients {
		out.Ingredients = append(out.Ingredients, ingredientJSON{Name: i.Name, Quantity: i.Quantity, Unit: i.Unit})
	}
	return out
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (in recipeInJSON) toInput() recipe.Input {
	out := recipe.Input{Title: in.Title, BaseServings: in.BaseServings, SourceURL: in.SourceURL, Tags: in.Tags, Steps: in.Steps}
	for _, i := range in.Ingredients {
		out.Ingredients = append(out.Ingredients, recipe.IngredientInput{Name: i.Name, Quantity: i.Quantity, Unit: i.Unit})
	}
	return out
}

func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.Recipes.List(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]recipeJSON, 0, len(list))
	for _, rc := range list {
		out = append(out, toRecipeJSON(rc))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	var in recipeInJSON
	if !decode(w, r, &in) {
		return
	}
	rc, err := s.deps.Recipes.Create(r.Context(), principalFrom(r).HouseholdID, in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRecipeJSON(rc))
}

func (s *Server) handleGetRecipe(w http.ResponseWriter, r *http.Request) {
	rc, err := s.deps.Recipes.Get(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecipeJSON(rc))
}

func (s *Server) handleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	var in recipeInJSON
	if !decode(w, r, &in) {
		return
	}
	rc, err := s.deps.Recipes.Update(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id"), in.toInput())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecipeJSON(rc))
}

func (s *Server) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Recipes.Delete(r.Context(), principalFrom(r).HouseholdID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
