// Package plan holds the weekly meal plan use cases.
package plan

import (
	"fmt"
	"time"

	"mealplan/internal/domain"
)

const dateLayout = "2006-01-02"

// ValidWeek accepts only a YYYY-MM-DD string that is a Monday and returns it as UTC midnight.
func ValidWeek(week string) (time.Time, error) {
	t, err := time.Parse(dateLayout, week)
	if err != nil || t.Format(dateLayout) != week {
		return time.Time{}, fmt.Errorf("%w: week must be a YYYY-MM-DD date", domain.ErrInvalid)
	}
	if t.Weekday() != time.Monday {
		return time.Time{}, fmt.Errorf("%w: week must start on a Monday", domain.ErrInvalid)
	}
	return t, nil
}

// WeekDates returns the seven dates of the week starting on the given Monday.
func WeekDates(week string) ([]string, error) {
	start, err := ValidWeek(week)
	if err != nil {
		return nil, err
	}
	out := make([]string, 7)
	for i := range out {
		out[i] = start.AddDate(0, 0, i).Format(dateLayout)
	}
	return out, nil
}
