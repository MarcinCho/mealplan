package assistant

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/product"
	"mealplan/internal/store"
	"mealplan/internal/units"
)

const (
	KindExisting = "existing"
	KindNew      = "new"
	KindNone     = "none"

	maxReason   = 300
	maxAliasLen = 200
)

// IngredientQuery is one ingredient to match; quantity and unit are only context for the model.
type IngredientQuery struct {
	Name     string
	Quantity *float64
	Unit     string
}

// NewProduct is a product the model proposes; the user reviews it before it is created.
type NewProduct struct {
	Name                      string
	Per100g                   domain.Macros
	PieceGrams, DensityGPerML *float64
	Aliases                   []string
}

// Suggestion is the proposal for one ingredient: an existing product, a new product or none.
type Suggestion struct {
	Name        string
	Kind        string // KindExisting | KindNew | KindNone
	ProductID   string
	ProductName string
	New         *NewProduct
	Reason      string
}

// DuplicateGroup proposes folding AbsorbIDs into KeepID.
type DuplicateGroup struct {
	KeepID    string
	AbsorbIDs []string
	Reason    string
}

type newProductReply struct {
	Name          string   `json:"name"`
	Kcal          *float64 `json:"kcal"`
	Protein       *float64 `json:"protein"`
	Carbs         *float64 `json:"carbs"`
	Fat           *float64 `json:"fat"`
	PieceGrams    *float64 `json:"pieceGrams"`
	DensityGPerML *float64 `json:"densityGPerMl"`
	Aliases       []string `json:"aliases"`
}

type matchReply struct {
	Suggestions *[]struct {
		Index      int              `json:"index"`
		Kind       string           `json:"kind"`
		ProductID  *string          `json:"productId"`
		NewProduct *newProductReply `json:"newProduct"`
		Reason     string           `json:"reason"`
	} `json:"suggestions"`
}

type duplicatesReply struct {
	Groups *[]struct {
		KeepID    string   `json:"keepId"`
		AbsorbIDs []string `json:"absorbIds"`
		Reason    string   `json:"reason"`
	} `json:"groups"`
}

func badReply(what string) error {
	return fmt.Errorf("%w: the model's answer is not valid %s JSON", ErrUnavailable, what)
}

// parseMatch turns the model's answer into one suggestion per query, in order. The answer is
// untrusted: unknown ids, unknown kinds, missing or repeated indexes and new products outside the
// allowed ranges all become KindNone; a "new" product that already exists becomes KindExisting.
// Queries proposed as the same new product (same normalized name) share the first proposal.
func parseMatch(raw string, qs []IngredientQuery, products []domain.Product) ([]Suggestion, error) {
	var r matchReply
	if err := json.Unmarshal([]byte(raw), &r); err != nil || r.Suggestions == nil {
		return nil, badReply("suggestions")
	}
	byID := make(map[string]domain.Product, len(products))
	for _, p := range products {
		byID[p.ID] = p
	}
	m := nutrition.NewMatcher(products)
	out := make([]Suggestion, len(qs))
	for i, q := range qs {
		out[i] = Suggestion{Name: q.Name, Kind: KindNone}
	}
	seen := make(map[int]bool, len(qs))
	proposed := map[string]*NewProduct{} // one proposal per new product name, shared by its queries
	for _, it := range *r.Suggestions {
		if it.Index < 0 || it.Index >= len(qs) || seen[it.Index] {
			continue
		}
		seen[it.Index] = true
		s := &out[it.Index]
		reason := clean(it.Reason, maxReason)
		switch it.Kind {
		case KindExisting:
			if it.ProductID == nil {
				continue
			}
			if p, ok := byID[*it.ProductID]; ok {
				s.Kind, s.ProductID, s.ProductName, s.Reason = KindExisting, p.ID, p.Name, reason
			}
		case KindNew:
			if it.NewProduct == nil {
				continue
			}
			if p, ok := existingNamed(m, it.NewProduct.Name); ok {
				s.Kind, s.ProductID, s.ProductName, s.Reason = KindExisting, p.ID, p.Name, reason
			} else if np, ok := proposeNew(m, it.NewProduct); ok {
				if first, dup := proposed[units.NormalizeName(np.Name)]; dup {
					np = first
				} else {
					proposed[units.NormalizeName(np.Name)] = np
				}
				s.Kind, s.New, s.Reason = KindNew, np, reason
			}
		case KindNone:
			s.Reason = reason
		}
	}
	return out, nil
}

// existingNamed finds the product whose name or alias equals name.
func existingNamed(m *nutrition.Matcher, name string) (domain.Product, bool) {
	if units.NormalizeName(name) == "" {
		return domain.Product{}, false
	}
	return m.Match(domain.Ingredient{Name: name})
}

// proposeNew validates a proposed product with the same rules as product.Create. Aliases that
// are empty, too long or already used by a product are dropped rather than failing the proposal.
func proposeNew(m *nutrition.Matcher, r *newProductReply) (*NewProduct, bool) {
	name := strings.Join(strings.Fields(r.Name), " ")
	if name == "" {
		return nil, false
	}
	own := units.NormalizeName(name)
	aliases := []string{}
	for _, a := range r.Aliases {
		n := units.NormalizeName(a)
		if n == "" || n == own || domain.HasControl(n) || utf8.RuneCountInString(n) > maxAliasLen || len(aliases) >= store.MaxAliases {
			continue
		}
		if _, taken := m.Match(domain.Ingredient{Name: n}); taken {
			continue
		}
		aliases = append(aliases, n)
	}
	p, err := product.Validate(product.Input{
		Name:          name,
		Per100g:       domain.Macros{Kcal: r.Kcal, Protein: r.Protein, Carbs: r.Carbs, Fat: r.Fat},
		PieceGrams:    positive(r.PieceGrams),
		DensityGPerML: positive(r.DensityGPerML),
		Aliases:       aliases,
	})
	if err != nil {
		return nil, false
	}
	if p.Aliases == nil {
		p.Aliases = []string{}
	}
	return &NewProduct{Name: p.Name, Per100g: p.Per100g, PieceGrams: p.PieceGrams, DensityGPerML: p.DensityGPerML, Aliases: p.Aliases}, true
}

// positive treats 0 as "not given": models write 0 for a weight or density that does not apply.
func positive(v *float64) *float64 {
	if v != nil && *v == 0 {
		return nil
	}
	return v
}

// parseDuplicates keeps only groups made of ids from the batch: foreign ids are dropped, the kept
// product is never also absorbed, a product appears in at most one (the first) group, and groups
// left with nothing to absorb are dropped.
func parseDuplicates(raw string, batch []string) ([]DuplicateGroup, error) {
	var r duplicatesReply
	if err := json.Unmarshal([]byte(raw), &r); err != nil || r.Groups == nil {
		return nil, badReply("duplicates")
	}
	inBatch := make(map[string]bool, len(batch))
	for _, id := range batch {
		inBatch[id] = true
	}
	used := map[string]bool{}
	out := []DuplicateGroup{}
	for _, g := range *r.Groups {
		if !inBatch[g.KeepID] || used[g.KeepID] {
			continue
		}
		var absorb []string
		mine := map[string]bool{g.KeepID: true}
		for _, id := range g.AbsorbIDs {
			if !inBatch[id] || used[id] || mine[id] {
				continue
			}
			mine[id] = true
			absorb = append(absorb, id)
		}
		if len(absorb) == 0 {
			continue
		}
		for id := range mine {
			used[id] = true
		}
		out = append(out, DuplicateGroup{KeepID: g.KeepID, AbsorbIDs: absorb, Reason: clean(g.Reason, maxReason)})
	}
	return out, nil
}

// clean collapses whitespace, drops control characters and shortens s to max runes.
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}
