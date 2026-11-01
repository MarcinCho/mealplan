package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrUnauthenticated    = errors.New("auth: unauthenticated")
	ErrForbidden          = errors.New("auth: forbidden")
	ErrSetupDone          = errors.New("auth: setup already done")
	ErrInviteInvalid      = errors.New("auth: invite invalid")
	ErrLoginTaken         = errors.New("auth: login taken")
	ErrInvalidInput       = errors.New("auth: invalid input")
)

type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return "auth: rate limited" }

type Config struct {
	SessionIdle time.Duration
	SessionMax  time.Duration
	InviteTTL   time.Duration
	Hasher      Hasher
}

type Service struct {
	st           *store.Store
	cfg          Config
	now          func() time.Time
	loginLimiter *Limiter
	ipLimiter    *Limiter
	dummyHash    string // verified against when the user does not exist, to equalize timing
}

func NewService(st *store.Store, cfg Config, now func() time.Time) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	dummy, err := cfg.Hasher.Hash("timing-equalization-password")
	if err != nil {
		return nil, err
	}
	return &Service{
		st: st, cfg: cfg, now: now,
		loginLimiter: NewLimiter(now, 5),
		ipLimiter:    NewLimiter(now, 30),
		dummyHash:    dummy,
	}, nil
}

type SessionGrant struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

type Principal struct {
	UserID        string
	Login         string
	DisplayName   string
	HouseholdID   string
	HouseholdName string
	SessionID     string
	CSRFToken     string
	Role          domain.Role
}

type SetupInput struct{ HouseholdName, Login, Password, DisplayName string }

type AcceptInput struct{ Login, Password, DisplayName string }

type InviteInfo struct {
	HouseholdName string
	Role          domain.Role
	ExpiresAt     time.Time
}

var loginRe = regexp.MustCompile(`^[a-z0-9._-]{3,64}$`)

func normalizeLogin(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validateProfile(login, displayName string) error {
	if !loginRe.MatchString(login) {
		return fmt.Errorf("%w: login must be 3 to 64 characters of a-z, 0-9, '.', '_' or '-'", ErrInvalidInput)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(displayName)); n < 1 || n > 64 {
		return fmt.Errorf("%w: display name must be 1 to 64 characters", ErrInvalidInput)
	}
	return nil
}

func (s *Service) audit(ctx context.Context, householdID, userID, typ, details string) {
	_ = s.st.InsertAudit(ctx, domain.AuditEvent{
		ID: NewID(), HouseholdID: householdID, UserID: userID, Type: typ, Details: details, At: s.now(),
	})
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	n, err := s.st.CountUsers(ctx)
	return n == 0, err
}

func (s *Service) Setup(ctx context.Context, in SetupInput) (SessionGrant, error) {
	login := normalizeLogin(in.Login)
	if err := validateProfile(login, in.DisplayName); err != nil {
		return SessionGrant{}, err
	}
	name := strings.TrimSpace(in.HouseholdName)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return SessionGrant{}, fmt.Errorf("%w: household name must be 1 to 100 characters", ErrInvalidInput)
	}
	if err := ValidatePassword(in.Password); err != nil {
		return SessionGrant{}, err
	}
	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return SessionGrant{}, err
	}
	now := s.now()
	h := domain.Household{ID: NewID(), Name: name, CreatedAt: now}
	u := domain.User{ID: NewID(), Login: login, PasswordHash: hash, DisplayName: strings.TrimSpace(in.DisplayName), CreatedAt: now}
	if err := s.st.CreateHouseholdWithAdmin(ctx, h, u, true); err != nil {
		if errors.Is(err, store.ErrSetupDone) {
			return SessionGrant{}, ErrSetupDone
		}
		return SessionGrant{}, err
	}
	s.audit(ctx, h.ID, u.ID, "setup", "")
	return s.startSession(ctx, u.ID)
}

func (s *Service) Login(ctx context.Context, login, password, ip string) (SessionGrant, error) {
	login = normalizeLogin(login)
	loginKey, ipKey := "login:"+login, "ip:"+ip
	for _, c := range []struct {
		l   *Limiter
		key string
	}{{s.loginLimiter, loginKey}, {s.ipLimiter, ipKey}} {
		if ok, wait := c.l.Allow(c.key); !ok {
			return SessionGrant{}, &RateLimitedError{RetryAfter: wait}
		}
	}

	u, err := s.st.UserByLogin(ctx, login)
	found := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return SessionGrant{}, err
	}

	ok := false
	if utf8.RuneCountInString(password) <= MaxPasswordLen { // never hash oversized input
		hash := s.dummyHash
		if found {
			hash = u.PasswordHash
		}
		match, verr := Verify(password, hash)
		ok = verr == nil && match && found
	}
	if !ok {
		s.loginLimiter.Fail(loginKey)
		s.ipLimiter.Fail(ipKey)
		s.audit(ctx, "", "", "login_failed", "ip="+ip)
		return SessionGrant{}, ErrInvalidCredentials
	}
	s.loginLimiter.Reset(loginKey)
	s.audit(ctx, "", u.ID, "login", "ip="+ip)
	return s.startSession(ctx, u.ID)
}

func (s *Service) startSession(ctx context.Context, userID string) (SessionGrant, error) {
	hid, _, err := s.st.FirstMembership(ctx, userID)
	if err != nil {
		return SessionGrant{}, err
	}
	raw, hash, err := NewToken()
	if err != nil {
		return SessionGrant{}, err
	}
	csrf, _, err := NewToken()
	if err != nil {
		return SessionGrant{}, err
	}
	now := s.now()
	sess := domain.Session{
		ID: NewID(), UserID: userID, HouseholdID: hid, TokenHash: hash, CSRFToken: csrf,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(s.cfg.SessionMax),
	}
	if err := s.st.CreateSession(ctx, sess); err != nil {
		return SessionGrant{}, err
	}
	return SessionGrant{Token: raw, CSRFToken: csrf, ExpiresAt: sess.ExpiresAt}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	sess, err := s.st.SessionByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.st.DeleteSession(ctx, sess.ID)
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	if rawToken == "" {
		return Principal{}, ErrUnauthenticated
	}
	sess, err := s.st.SessionByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	now := s.now()
	if !now.Before(sess.ExpiresAt) || !now.Before(sess.LastSeen.Add(s.cfg.SessionIdle)) {
		_ = s.st.DeleteSession(ctx, sess.ID)
		return Principal{}, ErrUnauthenticated
	}
	role, err := s.st.RoleOf(ctx, sess.UserID, sess.HouseholdID)
	if errors.Is(err, store.ErrNotFound) {
		_ = s.st.DeleteSession(ctx, sess.ID)
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	user, err := s.st.UserByID(ctx, sess.UserID)
	if err != nil {
		return Principal{}, err
	}
	hh, err := s.st.HouseholdByID(ctx, sess.HouseholdID)
	if err != nil {
		return Principal{}, err
	}
	if now.Sub(sess.LastSeen) > time.Minute {
		_ = s.st.TouchSession(ctx, sess.ID, now)
	}
	return Principal{
		UserID: user.ID, Login: user.Login, DisplayName: user.DisplayName, HouseholdID: hh.ID, HouseholdName: hh.Name,
		SessionID: sess.ID, CSRFToken: sess.CSRFToken, Role: role,
	}, nil
}

func requireAdmin(p Principal) error {
	if p.Role != domain.RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func (s *Service) CreateInvite(ctx context.Context, p Principal, role domain.Role) (string, domain.Invite, error) {
	if err := requireAdmin(p); err != nil {
		return "", domain.Invite{}, err
	}
	if !role.Valid() {
		return "", domain.Invite{}, fmt.Errorf("%w: role must be admin or member", ErrInvalidInput)
	}
	raw, hash, err := NewToken()
	if err != nil {
		return "", domain.Invite{}, err
	}
	inv := domain.Invite{
		ID: NewID(), HouseholdID: p.HouseholdID, TokenHash: hash, Role: role,
		CreatedBy: p.UserID, ExpiresAt: s.now().Add(s.cfg.InviteTTL),
	}
	if err := s.st.CreateInvite(ctx, inv); err != nil {
		return "", domain.Invite{}, err
	}
	s.audit(ctx, p.HouseholdID, p.UserID, "invite_created", "role="+string(role))
	return raw, inv, nil
}

func (s *Service) ListInvites(ctx context.Context, p Principal) ([]domain.Invite, error) {
	if err := requireAdmin(p); err != nil {
		return nil, err
	}
	return s.st.ListPendingInvites(ctx, p.HouseholdID, s.now())
}

func (s *Service) RevokeInvite(ctx context.Context, p Principal, id string) error {
	if err := requireAdmin(p); err != nil {
		return err
	}
	if err := s.st.RevokeInvite(ctx, p.HouseholdID, id); err != nil {
		return err
	}
	s.audit(ctx, p.HouseholdID, p.UserID, "invite_revoked", "")
	return nil
}

func (s *Service) usableInvite(ctx context.Context, rawToken string) (domain.Invite, error) {
	inv, err := s.st.InviteByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return domain.Invite{}, ErrInviteInvalid
	}
	if err != nil {
		return domain.Invite{}, err
	}
	if inv.UsedAt != nil || !s.now().Before(inv.ExpiresAt) {
		return domain.Invite{}, ErrInviteInvalid
	}
	return inv, nil
}

func (s *Service) InviteInfo(ctx context.Context, rawToken string) (InviteInfo, error) {
	inv, err := s.usableInvite(ctx, rawToken)
	if err != nil {
		return InviteInfo{}, err
	}
	hh, err := s.st.HouseholdByID(ctx, inv.HouseholdID)
	if err != nil {
		return InviteInfo{}, err
	}
	return InviteInfo{HouseholdName: hh.Name, Role: inv.Role, ExpiresAt: inv.ExpiresAt}, nil
}

func (s *Service) AcceptInvite(ctx context.Context, rawToken string, in AcceptInput) (SessionGrant, error) {
	login := normalizeLogin(in.Login)
	if err := validateProfile(login, in.DisplayName); err != nil {
		return SessionGrant{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return SessionGrant{}, err
	}
	inv, err := s.usableInvite(ctx, rawToken)
	if err != nil {
		return SessionGrant{}, err
	}
	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return SessionGrant{}, err
	}
	now := s.now()
	u := domain.User{ID: NewID(), Login: login, PasswordHash: hash, DisplayName: strings.TrimSpace(in.DisplayName), CreatedAt: now}
	hid, err := s.st.AcceptInvite(ctx, inv.ID, u, now)
	switch {
	case errors.Is(err, store.ErrInviteUnusable):
		return SessionGrant{}, ErrInviteInvalid
	case errors.Is(err, store.ErrConflict):
		return SessionGrant{}, ErrLoginTaken
	case err != nil:
		return SessionGrant{}, err
	}
	s.audit(ctx, hid, u.ID, "invite_accepted", "role="+string(inv.Role))
	return s.startSession(ctx, u.ID)
}

func (s *Service) Members(ctx context.Context, p Principal) ([]domain.Member, error) {
	return s.st.ListMembers(ctx, p.HouseholdID)
}
