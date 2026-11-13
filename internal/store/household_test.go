package store_test

import (
	"context"
	"errors"
	"testing"

	"mealplan/internal/store"
)

func TestPortionsPerMeal(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	if n, err := s.PortionsPerMeal(ctx, "hA"); err != nil || n != 2 {
		t.Fatalf("default: %d %v", n, err)
	}
	if err := s.SetPortionsPerMeal(ctx, "hA", 3); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.PortionsPerMeal(ctx, "hA"); n != 3 {
		t.Fatalf("set: %d", n)
	}
	if n, _ := s.PortionsPerMeal(ctx, "hB"); n != 2 {
		t.Fatalf("other household: %d", n)
	}
	if err := s.SetPortionsPerMeal(ctx, "nope", 3); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing household: %v", err)
	}
	if _, err := s.PortionsPerMeal(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing household read: %v", err)
	}
}

func TestAssistantSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedTwoHouseholds(t, s)
	if k, m, err := s.AssistantSettings(ctx, "hA"); err != nil || k != "" || m != "" {
		t.Fatalf("default: %q %q %v", k, m, err)
	}
	if err := s.SetAssistantSettings(ctx, "hA", "sk-1", "openai/gpt-4o"); err != nil {
		t.Fatal(err)
	}
	if k, m, _ := s.AssistantSettings(ctx, "hA"); k != "sk-1" || m != "openai/gpt-4o" {
		t.Fatalf("set: %q %q", k, m)
	}
	if k, m, _ := s.AssistantSettings(ctx, "hB"); k != "" || m != "" {
		t.Fatalf("other household: %q %q", k, m)
	}
	if err := s.SetAssistantSettings(ctx, "nope", "k", "m"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing household: %v", err)
	}
}
