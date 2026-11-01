// Package httpapi exposes the REST API under /api/v1 and mounts the static frontend.
package httpapi

import (
	"log/slog"
	"net/http"

	"mealplan/internal/auth"
	"mealplan/internal/config"
)

type Server struct {
	auth   *auth.Service
	cfg    config.Config
	static http.Handler
	log    *slog.Logger
}

func New(a *auth.Service, cfg config.Config, static http.Handler, log *slog.Logger) *Server {
	return &Server{auth: a, cfg: cfg, static: static, log: log}
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
