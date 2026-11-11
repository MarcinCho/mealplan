// Package httpapi exposes the REST API under /api/v1 and mounts the static frontend.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"mealplan/internal/assistant"
	"mealplan/internal/auth"
	"mealplan/internal/config"
	"mealplan/internal/mealprep"
	"mealplan/internal/nutrition"
	"mealplan/internal/pantry"
	"mealplan/internal/plan"
	"mealplan/internal/product"
	"mealplan/internal/recipe"
	"mealplan/internal/shopping"
)

// Deps carries the use-case services the handlers call.
type Deps struct {
	Recipes   *recipe.Service
	Products  *product.Service
	Plan      *plan.Service
	Shopping  *shopping.Service
	Nutrition *nutrition.Service
	Pantry    *pantry.Service
	MealPrep  *mealprep.Service
	Importer  Importer
	Assistant *assistant.Service
}

type Server struct {
	auth   *auth.Service
	deps   Deps
	cfg    config.Config
	static http.Handler
	log    *slog.Logger

	importLimit *limiter
}

func New(a *auth.Service, d Deps, cfg config.Config, static http.Handler, log *slog.Logger) *Server {
	return &Server{auth: a, deps: d, cfg: cfg, static: static, log: log, importLimit: newLimiter(10, time.Minute)}
}

type route struct {
	pattern string
	handler http.Handler
}

func (s *Server) routes() []route {
	return []route{
		{"GET /api/v1/auth/status", http.HandlerFunc(s.handleStatus)},
		{"POST /api/v1/auth/setup", s.public(s.handleSetup)},
		{"POST /api/v1/auth/login", s.public(s.handleLogin)},
		{"POST /api/v1/auth/logout", s.private(s.handleLogout)},
		{"GET /api/v1/auth/me", s.private(s.handleMe)},
		{"POST /api/v1/auth/invites", s.private(s.handleCreateInvite)},
		{"GET /api/v1/auth/invites", s.private(s.handleListInvites)},
		{"DELETE /api/v1/auth/invites/{id}", s.private(s.handleRevokeInvite)},
		{"GET /api/v1/auth/invitations/{token}", http.HandlerFunc(s.handleInviteInfo)},
		{"POST /api/v1/auth/invitations/{token}/accept", s.public(s.handleAcceptInvite)},
		{"GET /api/v1/household/members", s.private(s.handleMembers)},
		{"GET /api/v1/recipes", s.private(s.handleListRecipes)},
		{"POST /api/v1/recipes", s.private(s.handleCreateRecipe)},
		{"GET /api/v1/recipes/{id}", s.private(s.handleGetRecipe)},
		{"PUT /api/v1/recipes/{id}", s.private(s.handleUpdateRecipe)},
		{"DELETE /api/v1/recipes/{id}", s.private(s.handleDeleteRecipe)},
		{"POST /api/v1/recipes/import/url", s.private(s.handleImportURL)},
		{"POST /api/v1/recipes/import/text", s.private(s.handleImportText)},
		{"GET /api/v1/products", s.private(s.handleListProducts)},
		{"POST /api/v1/products", s.private(s.handleCreateProduct)},
		{"PUT /api/v1/products/{id}", s.private(s.handleUpdateProduct)},
		{"DELETE /api/v1/products/{id}", s.private(s.handleDeleteProduct)},
		{"POST /api/v1/products/{id}/assign", s.private(s.handleAssignProduct)},
		{"GET /api/v1/products/{id}/usage", s.private(s.handleProductUsage)},
		{"POST /api/v1/products/{id}/merge", s.private(s.handleMergeProducts)},
		{"PATCH /api/v1/household/members/{id}", s.private(s.handleChangeRole)},
		{"DELETE /api/v1/household/members/{id}", s.private(s.handleRemoveMember)},
		{"GET /api/v1/plan/{week}", s.private(s.handleGetWeek)},
		{"POST /api/v1/plan/{week}/entries", s.private(s.handleAddEntry)},
		{"PUT /api/v1/plan/{week}/entries/{id}", s.private(s.handleEditEntry)},
		{"DELETE /api/v1/plan/{week}/entries/{id}", s.private(s.handleRemoveEntry)},
		{"POST /api/v1/plan/{week}/copy", s.private(s.handleCopyWeek)},
		{"GET /api/v1/me/nutrition", s.private(s.handleGetNutrition)},
		{"PUT /api/v1/me/nutrition", s.private(s.handlePutNutrition)},
		{"GET /api/v1/me/nutrition/days", s.private(s.handleNutritionDays)},
		{"POST /api/v1/me/food-log", s.private(s.handleAddFoodLog)},
		{"DELETE /api/v1/me/food-log/{id}", s.private(s.handleDeleteFoodLog)},
		{"GET /api/v1/shopping", s.private(s.handleListShopping)},
		{"POST /api/v1/shopping/generate", s.private(s.handleGenerateShopping)},
		{"POST /api/v1/shopping/items", s.private(s.handleAddShoppingItem)},
		{"PATCH /api/v1/shopping/items/{id}", s.private(s.handlePatchShoppingItem)},
		{"DELETE /api/v1/shopping/items/{id}", s.private(s.handleDeleteShoppingItem)},
		{"POST /api/v1/shopping/move-to-pantry", s.private(s.handleMoveToPantry)},
		{"GET /api/v1/pantry", s.private(s.handleListPantry)},
		{"POST /api/v1/pantry", s.private(s.handleAddPantry)},
		{"POST /api/v1/pantry/review", s.private(s.handleReviewPantry)},
		{"PUT /api/v1/pantry/{id}", s.private(s.handleUpdatePantry)},
		{"DELETE /api/v1/pantry/{id}", s.private(s.handleDeletePantry)},
		{"GET /api/v1/prep/sessions", s.private(s.handleListSessions)},
		{"POST /api/v1/prep/sessions", s.private(s.handleCreateSession)},
		{"PUT /api/v1/prep/sessions/{id}", s.private(s.handleUpdateSession)},
		{"DELETE /api/v1/prep/sessions/{id}", s.private(s.handleDeleteSession)},
		{"POST /api/v1/prep/sessions/{id}/dishes", s.private(s.handleAddDish)},
		{"PUT /api/v1/prep/dishes/{id}", s.private(s.handleUpdateDish)},
		{"DELETE /api/v1/prep/dishes/{id}", s.private(s.handleDeleteDish)},
		{"POST /api/v1/prep/dishes/{id}/freeze", s.private(s.handleFreezeDish)},
		{"GET /api/v1/freezer", s.private(s.handleListFreezer)},
		{"POST /api/v1/freezer", s.private(s.handleAddFreezer)},
		{"PUT /api/v1/freezer/{id}", s.private(s.handleUpdateFreezer)},
		{"DELETE /api/v1/freezer/{id}", s.private(s.handleDeleteFreezer)},
		{"GET /api/v1/household/settings", s.private(s.handleGetHouseholdSettings)},
		{"PUT /api/v1/household/settings", s.private(s.handlePutHouseholdSettings)},
		{"GET /api/v1/household/assistant", s.private(s.handleGetAssistant)},
		{"PUT /api/v1/household/assistant", s.private(s.handlePutAssistant)},
		{"DELETE /api/v1/household/assistant", s.private(s.handleDeleteAssistantKey)},
		{"POST /api/v1/household/assistant/test", s.private(s.handleTestAssistant)},
		{"POST /api/v1/assistant/ingredients", s.private(s.handleAssistantIngredients)},
		{"GET /api/v1/assistant/ingredient-candidates", s.private(s.handleIngredientCandidates)},
		{"GET /api/v1/assistant/product-batches", s.private(s.handleProductBatches)},
		{"POST /api/v1/assistant/duplicates", s.private(s.handleAssistantDuplicates)},
	}
}

// Routes lists the registered API route patterns (used to keep openapi.yaml in sync).
func (s *Server) Routes() []string {
	var out []string
	for _, r := range s.routes() {
		out = append(out, r.pattern)
	}
	return out
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, r := range s.routes() {
		mux.Handle(r.pattern, r.handler)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})
	mux.Handle("/", s.static)
	return s.recoverer(s.securityHeaders(mux))
}
