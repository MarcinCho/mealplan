package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/importer"
	"mealplan/internal/plan"
	"mealplan/internal/store"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Code: code, Message: msg})
}

// fail maps service errors to HTTP responses. Unknown errors become 500 and are logged.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var rl *auth.RateLimitedError
	switch {
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rl.RetryAfter.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid login or password")
	case errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	case errors.Is(err, auth.ErrLastAdmin):
		writeError(w, http.StatusConflict, "last_admin", "the household must keep at least one admin")
	case errors.Is(err, auth.ErrSetupDone):
		writeError(w, http.StatusConflict, "setup_done", "setup has already been completed")
	case errors.Is(err, auth.ErrInviteInvalid):
		writeError(w, http.StatusNotFound, "invite_invalid", "invite is invalid, used or expired")
	case errors.Is(err, auth.ErrLoginTaken):
		writeError(w, http.StatusConflict, "login_taken", "login is already taken")
	case errors.Is(err, auth.ErrPasswordLength):
		writeError(w, http.StatusUnprocessableEntity, "password_length", "password must be 12 to 128 characters")
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, importer.ErrBlocked):
		writeError(w, http.StatusUnprocessableEntity, "import_blocked", "this address cannot be imported")
	case errors.Is(err, importer.ErrUnparsable):
		writeError(w, http.StatusUnprocessableEntity, "import_unparsable", "no recipe found in the source")
	case errors.Is(err, importer.ErrFetch):
		writeError(w, http.StatusUnprocessableEntity, "import_failed", "the page could not be fetched")
	case errors.Is(err, importer.ErrTimeout):
		writeError(w, http.StatusGatewayTimeout, "import_timeout", "the page took too long to respond")
	case errors.Is(err, plan.ErrPantry):
		s.log.Error("pantry update failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "pantry_failed", "Zapisano posiłek, ale nie udało się zaktualizować spiżarni. Spróbuj ponownie.")
	case errors.Is(err, domain.ErrInvalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "in_use", "resource is still in use")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "resource already exists or is not empty")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}
