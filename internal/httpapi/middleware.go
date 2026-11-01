package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"mealplan/internal/auth"
)

const (
	sessionCookie = "mealplan_session"
	csrfCookie    = "mealplan_csrf"
	csrfHeader    = "X-CSRF-Token"
)

type ctxKey int

const principalKey ctxKey = 1

func principalFrom(r *http.Request) auth.Principal {
	p, _ := r.Context().Value(principalKey).(auth.Principal)
	return p
}

// Script sources are strict. style-src allows 'unsafe-inline' because Vuetify injects its
// theme stylesheet at runtime; replace it with a nonce in a later stage (see docs/security.md).
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", csp)
		if s.cfg.CookieSecure {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "path", r.URL.Path, "panic", rec)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// public wraps handlers that need no session. They accept only JSON bodies, which blocks
// cross-site form posts (a "login CSRF") without needing a token before the user has a session.
func (s *Server) public(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
			return
		}
		h(w, r)
	})
}

// private wraps handlers that need a session; unsafe methods also need a matching CSRF token.
func (s *Server) private(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		raw := ""
		if err == nil {
			raw = c.Value
		}
		p, err := s.auth.Authenticate(r.Context(), raw)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			cc, cerr := r.Cookie(csrfCookie)
			hdr := r.Header.Get(csrfHeader)
			if cerr != nil || hdr == "" ||
				subtle.ConstantTimeCompare([]byte(hdr), []byte(cc.Value)) != 1 ||
				subtle.ConstantTimeCompare([]byte(hdr), []byte(p.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, "csrf", "missing or invalid CSRF token")
				return
			}
		}
		h(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	})
}
