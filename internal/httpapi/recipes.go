package httpapi

import (
	"net/http"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/recipe"
)

type ingredientJSON struct {
	Name      string   `json:"name"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
	Note      string   `json:"note"`
	ProductID *string  `json:"productId"`
}

type sourcesJSON struct {
	Kcal    string `json:"kcal"`
	Protein string `json:"protein"`
	Carbs   string `json:"carbs"`
	Fat     string `json:"fat"`
}

type nutritionPerPortionJSON struct {
	Values      macrosJSON  `json:"values"`
	Sources     sourcesJSON `json:"sources"`
	Approximate bool        `json:"approximate"`
	Unmatched   []string    `json:"unmatched"`
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
	Nutrition    macrosJSON       `json:"nutrition"`
	// NutritionPerPortion is set only by GET /recipes/{id}.
	NutritionPerPortion *nutritionPerPortionJSON `json:"nutritionPerPortion,omitempty"`
}

type recipeInJSON struct {
	Title        string           `json:"title"`
	BaseServings int              `json:"baseServings"`
	SourceURL    string           `json:"sourceUrl"`
	Tags         []string         `json:"tags"`
	Ingredients  []ingredientJSON `json:"ingredients"`
	Steps        []string         `json:"steps"`
	Nutrition    macrosJSON       `json:"nutrition"`
}

func toRecipeJSON(r domain.Recipe) recipeJSON {
	out := recipeJSON{ID: r.ID, Title: r.Title, BaseServings: r.BaseServings, SourceURL: r.SourceURL,
		Tags: orEmpty(r.Tags), Steps: orEmpty(r.Steps), Ingredients: []ingredientJSON{},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Nutrition: macrosJSON(r.Nutrition)}
	for _, i := range r.Ingredients {
		out.Ingredients = append(out.Ingredients, ingredientJSON{Name: i.Name, Quantity: i.Quantity, Unit: i.Unit, Note: i.Note, ProductID: nullable(i.ProductID)})
	}
	return out
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (in recipeInJSON) toInput() recipe.Input {
	out := recipe.Input{Title: in.Title, BaseServings: in.BaseServings, SourceURL: in.SourceURL, Tags: in.Tags, Steps: in.Steps, Nutrition: domain.Macros(in.Nutrition)}
	for _, i := range in.Ingredients {
		out.Ingredients = append(out.Ingredients, recipe.IngredientInput{Name: i.Name, Quantity: i.Quantity, Unit: i.Unit, Note: i.Note, ProductID: deref(i.ProductID)})
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
	products, err := s.deps.Products.All(r.Context(), principalFrom(r).HouseholdID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	n := nutrition.ForRecipe(rc, nutrition.NewMatcher(products))
	out := toRecipeJSON(rc)
	out.NutritionPerPortion = &nutritionPerPortionJSON{
		Values: macrosJSON(n.PerPortion),
		Sources: sourcesJSON{Kcal: string(n.Sources.Kcal), Protein: string(n.Sources.Protein),
			Carbs: string(n.Sources.Carbs), Fat: string(n.Sources.Fat)},
		Approximate: n.Approximate, Unmatched: orEmpty(n.Unmatched),
	}
	writeJSON(w, http.StatusOK, out)
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
