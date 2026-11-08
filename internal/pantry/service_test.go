package pantry_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/pantry"
	"mealplan/internal/product"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

type fixture struct {
	st    *store.Store
	svc   *pantry.Service
	prods *product.Service
	hid   string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st := store.New(storetest.Open(t))
	ctx := context.Background()
	for _, h := range []string{"h1", "h2"} {
		if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: h, Name: h}, domain.User{ID: "u" + h, Login: "l" + h, PasswordHash: "x", DisplayName: "U"}, false); err != nil {
			t.Fatal(err)
		}
	}
	prods := product.New(st, nil)
	return &fixture{st: st, svc: pantry.New(st, st, prods, nil), prods: prods, hid: "h1"}
}

func (f *fixture) product(name string) domain.Product {
	p, err := f.prods.Create(context.Background(), f.hid, product.Input{Name: name})
	if err != nil {
		panic(err)
	}
	return p
}

func TestAddMatchesSeedAndCreatesProduct(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	all, _ := f.prods.All(ctx, f.hid)
	n := len(all)
	it, err := f.svc.Add(ctx, f.hid, pantry.Input{Name: "mąka pszenna", Quantity: fp(1), Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if it.Quantity == nil || *it.Quantity != 1000 || it.Unit != "g" {
		t.Fatalf("want 1000 g, got %+v", it)
	}
	all, _ = f.prods.All(ctx, f.hid)
	if len(all) != n {
		t.Fatalf("seed product should be reused, products %d -> %d", n, len(all))
	}
	it2, err := f.svc.Add(ctx, f.hid, pantry.Input{Name: "Kasza bulgur XYZ", Quantity: fp(200), Unit: "g"})
	if err != nil {
		t.Fatal(err)
	}
	all, _ = f.prods.All(ctx, f.hid)
	if len(all) != n+1 || it2.ProductID == it.ProductID {
		t.Fatalf("new product expected: %d products, %+v", len(all), it2)
	}
	// adding the same product again accumulates
	it3, err := f.svc.Add(ctx, f.hid, pantry.Input{Name: "Mąka pszenna", Quantity: fp(500), Unit: "g"})
	if err != nil || *it3.Quantity != 1500 {
		t.Fatalf("accumulate: %v %+v", err, it3)
	}
}

func TestAddStaple(t *testing.T) {
	f := newFixture(t)
	it, err := f.svc.Add(context.Background(), f.hid, pantry.Input{Name: "Sól testowa", Unit: "g", Staple: true})
	if err != nil || !it.Staple || it.Quantity != nil {
		t.Fatalf("staple: %v %+v", err, it)
	}
}

func TestInputValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := f.product("Walidacja")
	cases := map[string]pantry.Input{
		"nil quantity non staple": {ProductID: p.ID, Unit: "g"},
		"zero quantity":           {ProductID: p.ID, Quantity: fp(0), Unit: "g"},
		"negative quantity":       {ProductID: p.ID, Quantity: fp(-1), Unit: "g"},
		"huge quantity":           {ProductID: p.ID, Quantity: fp(1e6 + 1), Unit: "g"},
		"empty unit":              {ProductID: p.ID, Quantity: fp(1)},
		"long unit":               {ProductID: p.ID, Quantity: fp(1), Unit: strings.Repeat("a", 31)},
		"bad date":                {ProductID: p.ID, Quantity: fp(1), Unit: "g", ExpiresOn: "2026-13-40"},
		"no product no name":      {Quantity: fp(1), Unit: "g"},
	}
	for name, in := range cases {
		if _, err := f.svc.Add(ctx, f.hid, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if _, err := f.svc.Add(ctx, f.hid, pantry.Input{ProductID: p.ID, Quantity: fp(1), Unit: "g", ExpiresOn: "2026-12-01"}); err != nil {
		t.Fatalf("valid: %v", err)
	}
	other, err := f.prods.Create(ctx, "h2", product.Input{Name: "Obcy"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, f.hid, pantry.Input{ProductID: other.ID, Quantity: fp(1), Unit: "g"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign product: want ErrNotFound, got %v", err)
	}
}

func TestUpdateDeleteAndReview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	it, err := f.svc.Add(ctx, f.hid, pantry.Input{Name: "Edycja", Quantity: fp(1), Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	up, err := f.svc.Update(ctx, f.hid, it.ID, pantry.Input{ProductID: it.ProductID, Quantity: fp(2), Unit: "kg", ExpiresOn: "2026-12-01"})
	if err != nil || *up.Quantity != 2000 || up.ExpiresOn != "2026-12-01" {
		t.Fatalf("update: %v %+v", err, up)
	}
	if _, err := f.svc.Update(ctx, "h2", it.ID, pantry.Input{ProductID: it.ProductID, Quantity: fp(1), Unit: "g"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	items, at, err := f.svc.List(ctx, f.hid)
	if err != nil || len(items) != 1 || at != nil {
		t.Fatalf("list: %v %+v %v", err, items, at)
	}
	ts, err := f.svc.MarkReviewed(ctx, f.hid)
	if err != nil || time.Since(ts) > time.Minute {
		t.Fatalf("review: %v %v", err, ts)
	}
	if _, at, _ = f.svc.List(ctx, f.hid); at == nil {
		t.Fatal("reviewedAt not stored")
	}
	if err := f.svc.Delete(ctx, f.hid, it.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx, f.hid, it.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}
