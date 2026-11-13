package assistant

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"unicode"
	"unicode/utf8"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/store"
	"mealplan/internal/units"
)

const (
	MaxIngredientQueries = 25 // ingredients per MatchIngredients call
	MaxDuplicateProducts = 80 // products per FindDuplicates call

	maxKey      = 200
	maxModel    = 100
	maxName     = 200
	maxUnit     = 50
	keyHintLen  = 4
	minHintable = 8 // shorter keys get no hint, so the hint is never most of the key
)

var modelPattern = regexp.MustCompile(`^[a-z0-9._-]+/[a-z0-9._:-]+$`)

// SettingsRepo stores the household's OpenRouter key and model and lists recipe ingredients.
type SettingsRepo interface {
	AssistantSettings(ctx context.Context, householdID string) (key, model string, err error)
	SetAssistantSettings(ctx context.Context, householdID, key, model string) error
	IngredientNames(ctx context.Context, householdID string) ([]store.IngredientRef, error)
}

// ProductLister lists every product of a household (product.Service.All).
type ProductLister interface {
	All(ctx context.Context, householdID string) ([]domain.Product, error)
}

// Service answers the product assistant's questions. It only proposes: nothing it returns is saved.
type Service struct {
	repo     SettingsRepo
	products ProductLister
	client   Client
}

func NewService(repo SettingsRepo, products ProductLister, c Client) *Service {
	return &Service{repo: repo, products: products, client: c}
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalid}, a...)...)
}

// Status says whether a key is set, its last four characters and the stored model, which is empty
// while the household has not chosen one (DefaultModel applies then). The key itself is never returned.
func (s *Service) Status(ctx context.Context, p auth.Principal) (configured bool, keyHint, model string, err error) {
	key, model, err := s.repo.AssistantSettings(ctx, p.HouseholdID)
	if err != nil {
		return false, "", "", err
	}
	if key == "" {
		return false, "", model, nil
	}
	if r := []rune(key); len(r) >= minHintable {
		keyHint = string(r[len(r)-keyHintLen:])
	}
	return true, keyHint, model, nil
}

// Configure sets the key and model; only a household admin may. A nil key keeps the current key,
// an empty key removes it. An empty model means DefaultModel.
func (s *Service) Configure(ctx context.Context, p auth.Principal, key *string, model string) error {
	if p.Role != domain.RoleAdmin {
		return auth.ErrForbidden
	}
	if key != nil {
		// The messages below never repeat the key.
		if utf8.RuneCountInString(*key) > maxKey {
			return invalid("the key must be at most %d characters", maxKey)
		}
		for _, r := range *key {
			if unicode.IsSpace(r) || unicode.IsControl(r) {
				return invalid("the key must not contain spaces or control characters")
			}
		}
	}
	if len(model) > maxModel || (model != "" && !modelPattern.MatchString(model)) {
		return invalid("the model must look like vendor/model and be at most %d characters", maxModel)
	}
	cur, _, err := s.repo.AssistantSettings(ctx, p.HouseholdID)
	if err != nil {
		return err
	}
	if key != nil {
		cur = *key
	}
	return s.repo.SetAssistantSettings(ctx, p.HouseholdID, cur, model)
}

// settings returns the key and the model to use, or ErrNotConfigured.
func (s *Service) settings(ctx context.Context, householdID string) (key, model string, err error) {
	key, model, err = s.repo.AssistantSettings(ctx, householdID)
	if err != nil {
		return "", "", err
	}
	if key == "" {
		return "", "", ErrNotConfigured
	}
	if model == "" {
		model = DefaultModel
	}
	return key, model, nil
}

// Test checks the key and the model with a one-ingredient request through the real matching
// prompt and schema, so a provider that rejects the schema fails here and not on first use.
func (s *Service) Test(ctx context.Context, p auth.Principal) error {
	key, model, err := s.settings(ctx, p.HouseholdID)
	if err != nil {
		return err
	}
	raw, err := s.client.Complete(ctx, key, model, matchSystem, matchPrompt(testQuery, nil), matchSchema)
	if err != nil {
		return err
	}
	_, err = parseMatch(raw, testQuery, nil)
	return err
}

func checkQuery(q IngredientQuery) error {
	if domain.HasControl(q.Name) || domain.HasControl(q.Unit) {
		return invalid("text must not contain control characters")
	}
	if n := utf8.RuneCountInString(units.NormalizeName(q.Name)); n == 0 || n > maxName {
		return invalid("ingredient name must be 1 to %d characters", maxName)
	}
	if utf8.RuneCountInString(q.Unit) > maxUnit {
		return invalid("unit must be at most %d characters", maxUnit)
	}
	if v := q.Quantity; v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
		return invalid("quantity must be a number")
	}
	return nil
}

// MatchIngredients proposes a product for each query, in order. Exact name or alias matches are
// answered locally (Reason "exact"); only the rest go to the model.
func (s *Service) MatchIngredients(ctx context.Context, p auth.Principal, qs []IngredientQuery) ([]Suggestion, error) {
	if len(qs) > MaxIngredientQueries {
		return nil, invalid("at most %d ingredients at a time", MaxIngredientQueries)
	}
	for _, q := range qs {
		if err := checkQuery(q); err != nil {
			return nil, err
		}
	}
	key, model, err := s.settings(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	products, err := s.products.All(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	m := nutrition.NewMatcher(products)
	out := make([]Suggestion, len(qs))
	var rest []IngredientQuery
	var restAt []int
	for i, q := range qs {
		if prod, ok := m.Match(domain.Ingredient{Name: q.Name}); ok {
			out[i] = Suggestion{Name: q.Name, Kind: KindExisting, ProductID: prod.ID, ProductName: prod.Name, Reason: "exact"}
			continue
		}
		rest = append(rest, q)
		restAt = append(restAt, i)
	}
	if len(rest) == 0 {
		return out, nil
	}
	raw, err := s.client.Complete(ctx, key, model, matchSystem, matchPrompt(rest, products), matchSchema)
	if err != nil {
		return nil, err
	}
	got, err := parseMatch(raw, rest, products)
	if err != nil {
		return nil, err
	}
	for j, sug := range got {
		out[restAt[j]] = sug
	}
	return out, nil
}

// FindDuplicates asks the model for duplicate groups among the given products (one batch).
// Ids that are not products of the household are ignored.
func (s *Service) FindDuplicates(ctx context.Context, p auth.Principal, productIDs []string) ([]DuplicateGroup, error) {
	if len(productIDs) > MaxDuplicateProducts {
		return nil, invalid("at most %d products at a time", MaxDuplicateProducts)
	}
	key, model, err := s.settings(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	all, err := s.products.All(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(productIDs))
	for _, id := range productIDs {
		want[id] = true
	}
	var batch []domain.Product
	var ids []string
	for _, prod := range all {
		if want[prod.ID] {
			batch = append(batch, prod)
			ids = append(ids, prod.ID)
		}
	}
	if len(batch) < 2 {
		return []DuplicateGroup{}, nil
	}
	raw, err := s.client.Complete(ctx, key, model, duplicatesSystem, duplicatesPrompt(batch), duplicatesSchema)
	if err != nil {
		return nil, err
	}
	return parseDuplicates(raw, ids)
}

// IngredientCandidates lists the recipe ingredients worth checking; it does not call the model.
func (s *Service) IngredientCandidates(ctx context.Context, p auth.Principal) ([]IngredientCandidate, error) {
	refs, err := s.repo.IngredientNames(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	products, err := s.products.All(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	return CandidateIngredients(refs, products), nil
}

// ProductBatches splits the household's possible duplicates into FindDuplicates batches.
func (s *Service) ProductBatches(ctx context.Context, p auth.Principal) ([][]string, error) {
	products, err := s.products.All(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	return CandidateProductBatches(products, MaxDuplicateProducts), nil
}
