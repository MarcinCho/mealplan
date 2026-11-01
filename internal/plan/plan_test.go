package plan_test

import (
	"errors"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/plan"
)

func TestValidWeek(t *testing.T) {
	if _, err := plan.ValidWeek("2026-11-02"); err != nil {
		t.Fatalf("monday must be valid: %v", err)
	}
	for _, bad := range []string{"", "2026-11-03", "2026-11-01", "11/02/2026", "2026-13-01", "2026-02-30", "abc", "2026-11-02T00:00:00Z"} {
		if _, err := plan.ValidWeek(bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ValidWeek(%q) error=%v want ErrInvalid", bad, err)
		}
	}
}

func TestWeekDates(t *testing.T) {
	d, err := plan.WeekDates("2026-11-02")
	if err != nil || len(d) != 7 || d[0] != "2026-11-02" || d[6] != "2026-11-08" {
		t.Fatalf("dates: %v %v", d, err)
	}
	d, _ = plan.WeekDates("2026-12-28")
	if d[6] != "2027-01-03" {
		t.Fatalf("year boundary: %v", d)
	}
}
