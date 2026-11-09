package nutrition

import (
	"math"
	"testing"

	"mealplan/internal/domain"
)

func TestConvert(t *testing.T) {
	plain := domain.Product{}
	flour := domain.Product{DensityGPerML: f(0.5)}
	egg := domain.Product{PieceGrams: f(60)}
	cases := []struct {
		q        float64
		from, to string
		p        domain.Product
		want     float64
		ok       bool
	}{
		{3, "łyżki", "ml", plain, 45, true},
		{2, "łyżeczka", "ml", plain, 10, true},
		{1, "szklanka", "l", plain, 0.25, true},
		{1.5, "kg", "g", plain, 1500, true},
		{2, "", "szt", plain, 2, true},
		{1, "szklanka", "g", plain, 0, false}, // no density: nothing assumed
		{2, "szklanki", "g", flour, 250, true},
		{100, "g", "ml", flour, 200, true},
		{2, "szt", "g", egg, 120, true},
		{180, "g", "szt", egg, 3, true},
		{2, "szt", "g", plain, 0, false},
		{1, "ząbek", "g", plain, 0, false},
		{1, "puszka", "szt", plain, 0, false},
	}
	for _, c := range cases {
		got, ok := Convert(c.q, c.from, c.to, c.p)
		if ok != c.ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%v %s -> %s: got %v %v, want %v %v", c.q, c.from, c.to, got, ok, c.want, c.ok)
		}
	}
}
