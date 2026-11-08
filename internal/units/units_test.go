package units_test

import (
	"errors"
	"testing"

	"mealplan/internal/units"
)

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"  Mąka   Pszenna ": "mąka pszenna",
		"JAJKA":             "jajka",
		"":                  "",
	}
	for in, want := range cases {
		if got := units.NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCanonUnit(t *testing.T) {
	cases := map[string]string{"G": "g", " kg ": "kg", "Szt.": "szt", "sztuki": "szt", "łyżka": "łyżka", "": ""}
	for in, want := range cases {
		if got := units.CanonUnit(in); got != want {
			t.Errorf("CanonUnit(%q)=%q want %q", in, got, want)
		}
	}
}

func TestToBase(t *testing.T) {
	cases := []struct {
		amount float64
		unit   string
		wantA  float64
		wantU  string
	}{
		{0.5, "kg", 500, "g"},
		{300, "g", 300, "g"},
		{1.5, "l", 1500, "ml"},
		{2, "szt", 2, "szt"},
		{3, "", 3, ""},
	}
	for _, c := range cases {
		a, u := units.ToBase(c.amount, c.unit)
		if a != c.wantA || u != c.wantU {
			t.Errorf("ToBase(%v,%q)=(%v,%q) want (%v,%q)", c.amount, c.unit, a, u, c.wantA, c.wantU)
		}
	}
}

func TestHumanize(t *testing.T) {
	cases := []struct {
		amount float64
		unit   string
		wantA  float64
		wantU  string
	}{
		{1500, "g", 1.5, "kg"},
		{999, "g", 999, "g"},
		{1000, "ml", 1, "l"},
		{250, "ml", 250, "ml"},
		{2, "szt", 2, "szt"},
	}
	for _, c := range cases {
		a, u := units.Humanize(c.amount, c.unit)
		if a != c.wantA || u != c.wantU {
			t.Errorf("Humanize(%v,%q)=(%v,%q) want (%v,%q)", c.amount, c.unit, a, u, c.wantA, c.wantU)
		}
	}
}

func TestScale(t *testing.T) {
	got, err := units.Scale(200, 3, 4)
	if err != nil || got != 150 {
		t.Fatalf("Scale(200,3,4)=%v,%v", got, err)
	}
	got, err = units.Scale(100, 1, 3)
	if err != nil || got != 33.333 {
		t.Fatalf("Scale(100,1,3)=%v,%v want 33.333", got, err)
	}
	for _, bad := range [][2]int{{0, 4}, {-1, 4}, {2, 0}, {2, -3}} {
		if _, err := units.Scale(1, bad[0], bad[1]); !errors.Is(err, units.ErrBadServings) {
			t.Errorf("Scale(1,%d,%d) error=%v want ErrBadServings", bad[0], bad[1], err)
		}
	}
}

func TestRound(t *testing.T) {
	if got := units.Round(0.1 + 0.2); got != 0.3 {
		t.Fatalf("Round(0.1+0.2)=%v", got)
	}
}

func TestCanonUnitPolishForms(t *testing.T) {
	cases := map[string]string{
		"łyżki": "łyżka", "łyżek": "łyżka", "Łyżkę": "łyżka", "łyż": "łyżka",
		"łyżeczki": "łyżeczka", "łyżeczek": "łyżeczka",
		"szklanki": "szklanka", "szklanek": "szklanka", "szkl": "szklanka",
		"ząbki": "ząbek", "ząbków": "ząbek",
		"plasterki": "plaster", "plasterków": "plaster",
		"szczypty": "szczypta", "szczypta": "szczypta",
		"opakowania": "opakowanie", "op": "opakowanie",
		"puszki": "puszka", "puszek": "puszka",
		"pęczki": "pęczek", "garści": "garść", "kostki": "kostka",
		"główki": "główka", "kromki": "kromka",
		"litra": "l", "litrów": "l", "kilo": "kg", "gr": "g", "ml": "ml", "mililitrów": "ml",
	}
	for in, want := range cases {
		if got := units.CanonUnit(in); got != want {
			t.Errorf("CanonUnit(%q)=%q want %q", in, got, want)
		}
	}
}
