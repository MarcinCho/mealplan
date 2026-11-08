package nutrition_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/product"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func fp(v float64) *float64 { return &v }

type fx struct {
	svc   *nutrition.Service
	st    *store.Store
	a, a2 auth.Principal
	b     auth.Principal
	prodA domain.Product
	prodB domain.Product
}

func newFx(t *testing.T) *fx {
	t.Helper()
	st := store.New(storetest.Open(t))
	ctx := context.Background()
	for _, h := range []struct{ hid, uid, login string }{{"h1", "u1", "marc"}, {"h2", "u3", "eve"}} {
		if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: h.hid, Name: h.hid}, domain.User{ID: h.uid, Login: h.login, PasswordHash: "x", DisplayName: h.login}, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.AddMember(ctx, domain.User{ID: "u2", Login: "ola", PasswordHash: "x", DisplayName: "ola"}, "h1", domain.RoleMember); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	pa := domain.Product{ID: "pa", HouseholdID: "h1", Name: "Jogurt", NormalizedName: "jogurt", Per100g: domain.Macros{Kcal: fp(60)}, CreatedAt: now, UpdatedAt: now}
	pb := domain.Product{ID: "pb", HouseholdID: "h2", Name: "Obcy", NormalizedName: "obcy", Per100g: domain.Macros{Kcal: fp(60)}, CreatedAt: now, UpdatedAt: now}
	if err := st.CreateProduct(ctx, pa); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProduct(ctx, pb); err != nil {
		t.Fatal(err)
	}
	return &fx{svc: nutrition.New(st, product.New(st, nil), func() time.Time { return now }), st: st,
		a: auth.Principal{UserID: "u1", HouseholdID: "h1"}, a2: auth.Principal{UserID: "u2", HouseholdID: "h1"},
		b: auth.Principal{UserID: "u3", HouseholdID: "h2"}, prodA: pa, prodB: pb}
}

func isInvalid(err error) bool { return errors.Is(err, domain.ErrInvalid) }

func TestSaveSettingsValidation(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	bad := []domain.Macros{{Kcal: fp(-1)}, {Kcal: fp(20001)}, {Protein: fp(2001)}, {Carbs: fp(-0.1)}, {Fat: fp(2001)}}
	for _, g := range bad {
		if err := f.svc.SaveSettings(ctx, f.a, true, g); !isInvalid(err) {
			t.Fatalf("goal %+v: %v", g, err)
		}
	}
	if err := f.svc.SaveSettings(ctx, f.a, true, domain.Macros{Kcal: fp(20000), Protein: fp(2000)}); err != nil {
		t.Fatal(err)
	}
	s, _ := f.svc.Settings(ctx, f.a)
	if !s.Enabled || *s.Goal.Kcal != 20000 {
		t.Fatalf("%+v", s)
	}
	other, _ := f.svc.Settings(ctx, f.a2)
	if other.Enabled || other.Goal.Kcal != nil {
		t.Fatalf("settings leaked: %+v", other)
	}
}

func TestAddLogValidationAndIsolation(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	for _, c := range []struct {
		date, unit string
		q          float64
	}{{"2026-13-01", "g", 1}, {"", "g", 1}, {"2026-11-03", "kg", 1}, {"2026-11-03", "g", 0}, {"2026-11-03", "g", -1}, {"2026-11-03", "g", 10001}} {
		if _, err := f.svc.AddLog(ctx, f.a, c.date, "pa", c.q, c.unit); !isInvalid(err) {
			t.Fatalf("%+v: %v", c, err)
		}
	}
	if _, err := f.svc.AddLog(ctx, f.a, "2026-11-03", "pb", 1, "g"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign product: %v", err)
	}
	if _, err := f.svc.AddLog(ctx, f.a, "2026-11-03", "nope", 1, "g"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown product: %v", err)
	}
	e, err := f.svc.AddLog(ctx, f.a, "2026-11-03", "pa", 10000, "g")
	if err != nil || e.ID == "" || e.UserID != "u1" || e.HouseholdID != "h1" {
		t.Fatalf("add: %v %+v", err, e)
	}
	if err := f.svc.DeleteLog(ctx, f.a2, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("other user delete: %v", err)
	}
	if err := f.svc.DeleteLog(ctx, f.b, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("other household delete: %v", err)
	}
	if err := f.svc.DeleteLog(ctx, f.a, e.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDaysRangeAndContent(t *testing.T) {
	f := newFx(t)
	ctx := context.Background()
	for _, r := range [][2]string{{"2026-11-03", "2026-11-02"}, {"x", "2026-11-02"}, {"2026-11-01", "2026-12-02"}, {"2026-11-01", "bad"}} {
		if _, err := f.svc.Days(ctx, f.a, r[0], r[1]); !isInvalid(err) {
			t.Fatalf("%v: %v", r, err)
		}
	}
	if days, err := f.svc.Days(ctx, f.a, "2026-11-01", "2026-11-02"); err != nil || len(days) != 0 {
		t.Fatalf("disabled must give no days: %d %v", len(days), err)
	}
	if err := f.svc.SaveSettings(ctx, f.a, true, domain.Macros{}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SaveSettings(ctx, f.a2, true, domain.Macros{}); err != nil {
		t.Fatal(err)
	}
	if days, err := f.svc.Days(ctx, f.a, "2026-11-01", "2026-12-01"); err != nil || len(days) != 31 {
		t.Fatalf("31 days: %d %v", len(days), err)
	}
	if _, err := f.svc.AddLog(ctx, f.a, "2026-11-03", "pa", 100, "g"); err != nil {
		t.Fatal(err)
	}
	days, err := f.svc.Days(ctx, f.a, "2026-11-02", "2026-11-04")
	if err != nil || len(days) != 3 || days[0].Date != "2026-11-02" {
		t.Fatalf("%v %+v", err, days)
	}
	if len(days[1].Log) != 1 || len(days[0].Log) != 0 || *days[1].Total.Kcal != 60 {
		t.Fatalf("day: %+v", days[1])
	}
	other, _ := f.svc.Days(ctx, f.a2, "2026-11-02", "2026-11-04")
	if len(other[1].Log) != 0 {
		t.Fatal("log leaked to another user")
	}
}
