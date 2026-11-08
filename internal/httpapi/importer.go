package httpapi

import (
	"context"
	"net/http"
	"sync"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/importer"
)

// Importer turns a link or pasted text into a recipe draft. It never saves anything.
type Importer interface {
	Import(ctx context.Context, src importer.Source) (importer.Draft, error)
}

type draftIngredientJSON struct {
	Raw        string   `json:"raw"`
	Name       string   `json:"name"`
	Quantity   *float64 `json:"quantity"`
	Unit       string   `json:"unit"`
	Note       string   `json:"note"`
	Confidence string   `json:"confidence"`
}

type draftJSON struct {
	Title       string                `json:"title"`
	Servings    int                   `json:"servings"`
	SourceURL   string                `json:"sourceUrl"`
	Tags        []string              `json:"tags"`
	Ingredients []draftIngredientJSON `json:"ingredients"`
	Steps       []string              `json:"steps"`
	Nutrition   macrosJSON            `json:"nutrition"`
	Warnings    []string              `json:"warnings"`
}

func toDraftJSON(d importer.Draft) draftJSON {
	out := draftJSON{Title: d.Title, Servings: d.Servings, SourceURL: d.SourceURL,
		Tags: orEmpty(d.Tags), Steps: orEmpty(d.Steps), Warnings: orEmpty(d.Warnings), Nutrition: macrosJSON(d.Nutrition),
		Ingredients: []draftIngredientJSON{}}
	for _, i := range d.Ingredients {
		out.Ingredients = append(out.Ingredients, draftIngredientJSON{Raw: i.Raw, Name: i.Name, Quantity: i.Quantity,
			Unit: i.Unit, Note: i.Note, Confidence: i.Confidence})
	}
	return out
}

type importURLJSON struct {
	URL string `json:"url"`
}

type importTextJSON struct {
	Text string `json:"text"`
}

func (s *Server) handleImportURL(w http.ResponseWriter, r *http.Request) {
	var in importURLJSON
	if !decode(w, r, &in) {
		return
	}
	if ok, retry := s.importLimit.allow(principalFrom(r).UserID); !ok {
		s.fail(w, r, &auth.RateLimitedError{RetryAfter: retry})
		return
	}
	d, err := s.deps.Importer.Import(r.Context(), importer.Source{Kind: importer.KindURL, URL: in.URL})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDraftJSON(d))
}

func (s *Server) handleImportText(w http.ResponseWriter, r *http.Request) {
	var in importTextJSON
	if !decode(w, r, &in) {
		return
	}
	d, err := s.deps.Importer.Import(r.Context(), importer.Source{Kind: importer.KindText, Text: in.Text})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDraftJSON(d))
}

// limiter allows max calls per key in a sliding window. It keeps the server from being used to probe the network.
type limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
	now    func() time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{hits: map[string][]time.Time{}, max: max, window: window, now: time.Now}
}

func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(now.Add(-l.window)) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false, kept[0].Add(l.window).Sub(now)
	}
	l.hits[key] = append(kept, now)
	return true, 0
}
