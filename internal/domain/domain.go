// Package domain holds the core types shared across packages.
package domain

import (
	"errors"
	"time"
)

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

// ErrInvalid marks input rejected by validation; wrap it with the reason.
var ErrInvalid = errors.New("invalid input")

type Ingredient struct {
	Name           string
	NormalizedName string
	Quantity       *float64
	Unit           string
	Note           string
}

type Recipe struct {
	ID           string
	HouseholdID  string
	Title        string
	BaseServings int
	SourceURL    string
	Tags         []string
	Ingredients  []Ingredient
	Steps        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Slot string

const (
	SlotBreakfast Slot = "breakfast"
	SlotLunch     Slot = "lunch"
	SlotDinner    Slot = "dinner"
	SlotSnack     Slot = "snack"
)

func (s Slot) Valid() bool {
	return s == SlotBreakfast || s == SlotLunch || s == SlotDinner || s == SlotSnack
}

// PlanEntry is one dish in the week grid. Date is YYYY-MM-DD. Exactly one of RecipeID and
// FreeText is set; an empty CookUserID means nobody is assigned.
type PlanEntry struct {
	ID          string
	HouseholdID string
	Date        string
	Slot        Slot
	RecipeID    string
	FreeText    string
	Servings    int
	CookUserID  string
	Done        bool
}

// ShoppingItem belongs to the week starting on Week (YYYY-MM-DD, a Monday).
type ShoppingItem struct {
	ID             string
	HouseholdID    string
	Week           string
	Name           string
	NormalizedName string
	Quantity       *float64
	Unit           string
	Checked        bool
	Manual         bool
}
