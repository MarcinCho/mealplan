package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
)

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) setSession(w http.ResponseWriter, g auth.SessionGrant) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: g.Token, Path: "/", Expires: g.ExpiresAt,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: g.CSRFToken, Path: "/", Expires: g.ExpiresAt,
		HttpOnly: false, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: name == sessionCookie, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	need, err := s.auth.SetupRequired(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"setupRequired": need, "mode": string(s.cfg.Mode)})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		HouseholdName string `json:"householdName"`
		Login         string `json:"login"`
		Password      string `json:"password"`
		DisplayName   string `json:"displayName"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := s.auth.Setup(r.Context(), auth.SetupInput{
		HouseholdName: in.HouseholdName, Login: in.Login, Password: in.Password, DisplayName: in.DisplayName})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSession(w, g)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := s.auth.Login(r.Context(), in.Login, in.Password, clientIP(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSession(w, g)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(r.Context(), c.Value); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":      map[string]string{"id": p.UserID, "login": p.Login, "displayName": p.DisplayName},
		"household": map[string]string{"id": p.HouseholdID, "name": p.HouseholdName},
		"role":      string(p.Role),
		"csrfToken": p.CSRFToken,
	})
}

type inviteDTO struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	role := domain.Role(in.Role)
	if in.Role == "" {
		role = domain.RoleMember
	}
	raw, inv, err := s.auth.CreateInvite(r.Context(), principalFrom(r), role)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": inv.ID, "token": raw, "role": string(inv.Role), "expiresAt": inv.ExpiresAt, "path": "/invite/" + raw,
	})
}

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	list, err := s.auth.ListInvites(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]inviteDTO, 0, len(list))
	for _, inv := range list {
		out = append(out, inviteDTO{ID: inv.ID, Role: string(inv.Role), ExpiresAt: inv.ExpiresAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.RevokeInvite(r.Context(), principalFrom(r), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleInviteInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.auth.InviteInfo(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"householdName": info.HouseholdName, "role": string(info.Role), "expiresAt": info.ExpiresAt,
	})
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login       string `json:"login"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := s.auth.AcceptInvite(r.Context(), r.PathValue("token"),
		auth.AcceptInput{Login: in.Login, Password: in.Password, DisplayName: in.DisplayName})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSession(w, g)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.auth.Members(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]map[string]string, 0, len(members))
	for _, m := range members {
		out = append(out, map[string]string{
			"id": m.User.ID, "login": m.User.Login, "displayName": m.User.DisplayName, "role": string(m.Role),
		})
	}
	writeJSON(w, http.StatusOK, out)
}
