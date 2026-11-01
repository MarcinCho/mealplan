package domain_test

import (
	"testing"

	"mealplan/internal/domain"
)

func TestHasControl(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"zwykły tekst ąęś", false},
		{"", false},
		{"a\tb\nc\r\nd", false},
		{"a\x00b", true},
		{"\x01", true},
		{"a\x08b", true},
		{"\x1b[0m", true},
		{"a\x7fb", true},
		{"emoji 🍲", false},
	}
	for _, c := range cases {
		if got := domain.HasControl(c.in); got != c.want {
			t.Errorf("HasControl(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
