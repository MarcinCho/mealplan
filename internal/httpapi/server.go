// Package httpapi exposes the REST API under /api/v1 and mounts the static frontend.
package httpapi

import (
	"log/slog"
	"net/http"

	"mealplan/internal/auth"
	"mealplan/internal/config"
	"mealplan/internal/plan"
	"mealplan/internal/recipe"
)

// Deps carries the use-case services the handlers call.
type Deps struct {
	Recipes *recipe.Service
	Plan    *plan.Service
}

type Server struct {
	auth   *auth.Service
	deps   Deps
	cfg    config.Config
	static http.Handler
	log    *slog.Logger
}

func New(a *auth.Service, d Deps, cfg config.Config, static http.Handler, log *slog.Logger) *Server {
	return &Server{auth: a, deps: d, cfg: cfg, static: static, log: log}
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
		{"PATCH /api/v1/household/members/{id}", s.private(s.handleChangeRole)},
		{"DELETE /api/v1/household/members/{id}", s.private(s.handleRemoveMember)},
		{"GET /api/v1/plan/{week}", s.private(s.handleGetWeek)},
		{"POST /api/v1/plan/{week}/entries", s.private(s.handleAddEntry)},
		{"PUT /api/v1/plan/{week}/entries/{id}", s.private(s.handleEditEntry)},
		{"DELETE /api/v1/plan/{week}/entries/{id}", s.private(s.handleRemoveEntry)},
		{"POST /api/v1/plan/{week}/copy", s.private(s.handleCopyWeek)},
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
