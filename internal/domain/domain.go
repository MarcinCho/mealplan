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
	ProductID      string // remembered product choice, "" = none
}

// Macros holds nutrition values; nil means unknown.
type Macros struct {
	Kcal, Protein, Carbs, Fat *float64
}

type Product struct {
	ID, HouseholdID, Name, NormalizedName string
	Per100g                               Macros
	PieceGrams                            *float64
	DensityGPerML                         *float64
	Aliases                               []string
	CreatedAt, UpdatedAt                  time.Time
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
	Nutrition    Macros // given/manual values per portion
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
	HaveQuantity   *float64
	Moved          bool
	ProductID      string // product the generated line matched, "" for manual or unmatched items
}

// PantryItem is household stock of a product in one unit. Quantity is nil for a staple.
type PantryItem struct {
	ID, HouseholdID, ProductID, Name string
	Quantity                         *float64
	Unit                             string
	ExpiresOn                        string // "" or YYYY-MM-DD
	Staple                           bool
	UpdatedAt                        time.Time
}

// PantryMovement is a signed change of stock caused by a plan entry.
type PantryMovement struct {
	EntryID, ProductID, Unit string
	Amount                   float64
}

// NutritionSettings are per-user, per-household nutrition preferences.
type NutritionSettings struct {
	UserID, HouseholdID string
	Enabled             bool
	Goal                Macros
}

// FoodLogEntry is one logged product amount; Unit is "g" or "szt".
type FoodLogEntry struct {
	ID, HouseholdID, UserID, Date, ProductID string
	Quantity                                 float64
	Unit                                     string
	CreatedAt                                time.Time
}
