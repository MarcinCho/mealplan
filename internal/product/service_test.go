package product_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition/seed"
	"mealplan/internal/product"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func fp(v float64) *float64 { return &v }

func setup(t *testing.T) (*product.Service, *store.Store) {
	t.Helper()
	st := store.New(storetest.Open(t))
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	for _, h := range []string{"hA", "hB"} {
		if err := st.CreateHouseholdWithAdmin(context.Background(), domain.Household{ID: h, Name: h, CreatedAt: now},
			domain.User{ID: "u" + h, Login: "l" + h, PasswordHash: "x", DisplayName: "d", CreatedAt: now}, false); err != nil {
			t.Fatal(err)
		}
	}
	return product.New(st, func() time.Time { return now }), st
}

func valid() product.Input {
	return product.Input{Name: "  Mleko  3,2% ", Per100g: domain.Macros{Kcal: fp(60), Protein: fp(3), Carbs: fp(5), Fat: fp(3)}, Aliases: []string{" Mleko UHT ", "mleko uht", "Mleczko"}}
}

func TestCreateNormalizesAndStores(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	p, err := s.Create(ctx, "hA", valid())
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" || p.HouseholdID != "hA" || p.Name != "Mleko  3,2%" || p.NormalizedName != "mleko 3,2%" {
		t.Fatalf("%+v", p)
	}
	if len(p.Aliases) != 2 || p.Aliases[0] != "mleko uht" || p.Aliases[1] != "mleczko" {
		t.Fatalf("aliases: %v", p.Aliases)
	}
}

func TestValidation(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	cases := map[string]func(*product.Input){
		"empty name":  func(i *product.Input) { i.Name = "  " },
		"long name":   func(i *product.Input) { i.Name = strings.Repeat("ż", 201) },
		"control":     func(i *product.Input) { i.Name = "a\x00b" },
		"negative":    func(i *product.Input) { i.Per100g.Fat = fp(-1) },
		"kcal 901":    func(i *product.Input) { i.Per100g.Kcal = fp(901) },
		"protein 101": func(i *product.Input) { i.Per100g.Protein = fp(101) },
		"sum over 100": func(i *product.Input) {
			i.Per100g = domain.Macros{Kcal: fp(1), Protein: fp(50), Carbs: fp(40), Fat: fp(11)}
		},
		"piece zero":  func(i *product.Input) { i.PieceGrams = fp(0) },
		"piece 5001":  func(i *product.Input) { i.PieceGrams = fp(5001) },
		"density 0.2": func(i *product.Input) { i.DensityGPerML = fp(0.2) },
		"density 2.1": func(i *product.Input) { i.DensityGPerML = fp(2.1) },
		"21 aliases": func(i *product.Input) {
			i.Aliases = make([]string, 21)
			for k := range i.Aliases {
				i.Aliases[k] = string(rune('a' + k))
			}
		},
		"long alias":    func(i *product.Input) { i.Aliases = []string{strings.Repeat("a", 201)} },
		"empty alias":   func(i *product.Input) { i.Aliases = []string{"  "} },
		"alias control": func(i *product.Input) { i.Aliases = []string{"a\x01"} },
	}
	for name, mut := range cases {
		in := valid()
		mut(&in)
		if _, err := s.Create(ctx, "hA", in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	ok := valid()
	ok.Per100g = domain.Macros{Kcal: fp(900), Protein: fp(30), Carbs: fp(30), Fat: fp(40)}
	ok.PieceGrams, ok.DensityGPerML = fp(5000), fp(2)
	if _, err := s.Create(ctx, "hA", ok); err != nil {
		t.Fatalf("boundary values: %v", err)
	}
}

func TestAliasMustNotEqualOtherProductName(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, "hA", product.Input{Name: "Ser"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "hA", product.Input{Name: "Twaróg", Aliases: []string{"SER"}}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("create: %v", err)
	}
	other, err := s.Create(ctx, "hA", product.Input{Name: "Masło"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, "hA", other.ID, product.Input{Name: "Masło", Aliases: []string{"ser"}}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("update: %v", err)
	}
	// Same alias in another household is fine, and an alias equal to the own name is harmless.
	if _, err := s.Create(ctx, "hB", product.Input{Name: "Masło", Aliases: []string{"ser", "masło"}}); err != nil {
		t.Fatalf("other household: %v", err)
	}
	if _, err := s.Create(ctx, "hA", product.Input{Name: "Ser"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	p, _ := s.Create(ctx, "hA", valid())
	in := valid()
	in.Name = "Mleko UHT"
	in.Aliases = nil
	u, err := s.Update(ctx, "hA", p.ID, in)
	if err != nil || u.Name != "Mleko UHT" || len(u.Aliases) != 0 || !u.CreatedAt.Equal(p.CreatedAt) {
		t.Fatalf("%+v %v", u, err)
	}
	if _, err := s.Update(ctx, "hB", p.ID, in); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if err := s.Delete(ctx, "hB", p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if err := s.Delete(ctx, "hA", p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestListSeedsOnceAndFilters(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	want, err := seed.Products()
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.List(ctx, "hA", "")
	if err != nil || len(all) != len(want) {
		t.Fatalf("seeded %d want %d (%v)", len(all), len(want), err)
	}
	if all[0].ID == "" || all[0].HouseholdID != "hA" {
		t.Fatalf("%+v", all[0])
	}
	again, _ := s.All(ctx, "hA")
	if len(again) != len(want) {
		t.Fatalf("reseeded: %d", len(again))
	}
	// filter by name and alias, case-insensitively
	target := all[0]
	got, _ := s.List(ctx, "hA", strings.ToUpper(target.Name))
	found := false
	for _, p := range got {
		found = found || p.ID == target.ID
	}
	if !found || len(got) > len(all) {
		t.Fatalf("name filter: %d", len(got))
	}
	for _, p := range all {
		if len(p.Aliases) > 0 {
			got, _ = s.List(ctx, "hA", strings.ToUpper(p.Aliases[0]))
			found = false
			for _, g := range got {
				found = found || g.ID == p.ID
			}
			if !found {
				t.Fatalf("alias filter %q missed %s", p.Aliases[0], p.Name)
			}
			break
		}
	}
	if none, _ := s.List(ctx, "hA", "zzzz-nope-zzzz"); len(none) != 0 {
		t.Fatalf("no-match: %d", len(none))
	}
	// Deleting everything must not reseed.
	for _, p := range all {
		if err := s.Delete(ctx, "hA", p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if left, _ := s.All(ctx, "hA"); len(left) != 0 {
		t.Fatalf("reseeded after wipe: %d", len(left))
	}
}
