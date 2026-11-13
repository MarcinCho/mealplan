package assistant

import (
	"context"
	"encoding/json"
	"sync"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

func fp(v float64) *float64 { return &v }

// fakeClient records calls and answers with reply (or an empty JSON object).
type fakeClient struct {
	mu      sync.Mutex
	calls   int
	users   []string
	systems []string
	schemas []json.RawMessage
	key     string
	model   string
	reply   func(user string) (string, error)
}

func (f *fakeClient) Complete(_ context.Context, key, model, system, user string, schema json.RawMessage) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.users = append(f.users, user)
	f.systems = append(f.systems, system)
	f.schemas = append(f.schemas, schema)
	f.key, f.model = key, model
	if f.reply == nil {
		return `{}`, nil
	}
	return f.reply(user)
}

// fakeRepo keeps the assistant settings and ingredient list of every household in memory.
type fakeRepo struct {
	mu       sync.Mutex
	settings map[string][2]string
	refs     map[string][]store.IngredientRef
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{settings: map[string][2]string{}, refs: map[string][]store.IngredientRef{}}
}

func (r *fakeRepo) AssistantSettings(_ context.Context, householdID string) (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.settings[householdID]
	return s[0], s[1], nil
}

func (r *fakeRepo) SetAssistantSettings(_ context.Context, householdID, key, model string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings[householdID] = [2]string{key, model}
	return nil
}

func (r *fakeRepo) IngredientNames(_ context.Context, householdID string) ([]store.IngredientRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.refs[householdID], nil
}

type fakeProducts []domain.Product

func (f fakeProducts) All(context.Context, string) ([]domain.Product, error) { return f, nil }
