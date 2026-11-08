package nutrition

import (
	"math"
	"testing"

	"mealplan/internal/domain"
)

func f(v float64) *float64 { return &v }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestGrams(t *testing.T) {
	none := domain.Product{}
	jajko := domain.Product{PieceGrams: f(50)}
	olej := domain.Product{DensityGPerML: f(0.92)}
	mleko := domain.Product{DensityGPerML: f(1.03)}
	cases := []struct {
		name string
		q    float64
		unit string
		p    domain.Product
		want float64
		ok   bool
	}{
		{"g", 250, "g", none, 250, true},
		{"kg", 0.5, "kg", none, 500, true},
		{"l milk", 1, "l", mleko, 1030, true},
		{"ml no density", 200, "ml", none, 200, true},
		{"łyżka oil", 2, "łyżka", olej, 27.6, true},
		{"łyżeczka", 1, "łyżeczka", none, 5, true},
		{"szklanka", 1, "szklanka", none, 250, true},
		{"szt egg", 3, "szt", jajko, 150, true},
		{"ząbek no weight", 2, "ząbek", none, 0, false},
		{"opakowanie", 1, "opakowanie", domain.Product{PieceGrams: f(400)}, 400, true},
		{"szczypta", 1, "szczypta", none, 0.5, true},
		{"garść", 1, "garść", none, 30, true},
		{"cup", 1, "cup", none, 0, false},
	}
	for _, c := range cases {
		got, ok := Grams(c.q, c.unit, c.p)
		if ok != c.ok || (ok && !near(got, c.want)) {
			t.Errorf("%s: got (%v,%v) want (%v,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}
