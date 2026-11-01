// Package domain holds the core types shared across packages.
package domain

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) Valid() bool { return r == RoleAdmin || r == RoleMember }

type Household struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type User struct {
	ID           string
	Login        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
}

type Member struct {
	User User
	Role Role
}

type Invite struct {
	ID          string
	HouseholdID string
	TokenHash   string
	Role        Role
	CreatedBy   string
	ExpiresAt   time.Time
	UsedAt      *time.Time
}

type Session struct {
	ID          string
	UserID      string
	HouseholdID string
	TokenHash   string
	CSRFToken   string
	CreatedAt   time.Time
	LastSeen    time.Time
	ExpiresAt   time.Time
}

type AuditEvent struct {
	ID          string
	HouseholdID string
	UserID      string
	Type        string
	Details     string
	At          time.Time
}
