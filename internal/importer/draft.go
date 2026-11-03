// Package importer turns a link or pasted text into a recipe Draft that the user reviews before saving.
package importer

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"mealplan/internal/domain"
)

const (
	KindURL  = "url"
	KindText = "text"

	ConfOK       = "ok"
	ConfGuess    = "guess"
	ConfUnparsed = "unparsed"

	MaxTextBytes = 100 << 10

	maxTitle       = 200
	maxServings    = 100
	maxIngredients = 200
	maxSteps       = 100
	maxStepLen     = 2000
	maxTags        = 20
	maxTagLen      = 40
	maxURL         = 2000
	maxQuantity    = 1e6
)

var (
	ErrBlocked    = errors.New("importer: address not allowed")
	ErrTimeout    = errors.New("importer: timed out")
	ErrFetch      = errors.New("importer: page could not be fetched")
	ErrUnparsable = errors.New("importer: no recipe found")
)

type Source struct {
	Kind string
	URL  string
	Text string
}

type IngredientLine struct {
	Raw        string
	Name       string
	Quantity   *float64
	Unit       string
	Note       string
	Confidence string
}

type Draft struct {
	Title       string
	Servings    int // 0 means unknown
	SourceURL   string
	Tags        []string
	Ingredients []IngredientLine
	Steps       []string
	Warnings    []string
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalid, fmt.Sprintf(format, a...))
}

func (d *Draft) warn(format string, a ...any) {
	d.Warnings = append(d.Warnings, fmt.Sprintf(format, a...))
}

func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// finish enforces the recipe limits so a reviewed draft can always be saved, and reports anything it had to cut.
func (d *Draft) finish() {
	d.Title = strings.TrimSpace(d.Title)
	if utf8.RuneCountInString(d.Title) > maxTitle {
		d.Title = cut(d.Title, maxTitle)
		d.warn("Tytuł został skrócony do %d znaków.", maxTitle)
	}
	if d.Servings < 0 || d.Servings > maxServings {
		d.Servings = 0
	}
	if d.Servings == 0 {
		d.warn("Nie znaleziono liczby porcji, ustaw ją ręcznie.")
	}
	if d.SourceURL = strings.TrimSpace(d.SourceURL); len(d.SourceURL) > maxURL ||
		!(strings.HasPrefix(d.SourceURL, "http://") || strings.HasPrefix(d.SourceURL, "https://")) {
		d.SourceURL = ""
	}

	if len(d.Ingredients) == 0 {
		d.warn("Nie znaleziono składników.")
	}
	if len(d.Ingredients) > maxIngredients {
		d.Ingredients = d.Ingredients[:maxIngredients]
		d.warn("Pominięto składniki ponad limit %d.", maxIngredients)
	}

	steps := make([]string, 0, len(d.Steps))
	for _, s := range d.Steps {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > maxStepLen {
			s = cut(s, maxStepLen)
			d.warn("Jeden z kroków został skrócony do %d znaków.", maxStepLen)
		}
		steps = append(steps, s)
	}
	if len(steps) == 0 {
		d.warn("Nie znaleziono kroków.")
	}
	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
		d.warn("Pominięto kroki ponad limit %d.", maxSteps)
	}
	d.Steps = steps

	seen := map[string]bool{}
	tags := make([]string, 0, len(d.Tags))
	for _, tg := range d.Tags {
		tg = strings.ToLower(strings.TrimSpace(tg))
		if tg == "" || utf8.RuneCountInString(tg) > maxTagLen || seen[tg] || len(tags) == maxTags {
			continue
		}
		seen[tg] = true
		tags = append(tags, tg)
	}
	d.Tags = tags

	if d.Ingredients == nil {
		d.Ingredients = []IngredientLine{}
	}
	if d.Warnings == nil {
		d.Warnings = []string{}
	}
}
