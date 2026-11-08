package importer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mealplan/internal/domain"
)

func serve(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestImportURLReturnsFinishedDraftWithSourceURL(t *testing.T) {
	srv := serve(t, fixture(t, "jsonld_plain.html"))
	s := &Service{fetcher: newTestFetcher()}
	d, err := s.Import(context.Background(), Source{Kind: KindURL, URL: " " + srv.URL + "/r "})
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Naleśniki" || d.SourceURL != srv.URL+"/r" || len(d.Warnings) != 0 {
		t.Errorf("%+v", d)
	}
}

func TestImportURLPageWithoutRecipeIsUnparsable(t *testing.T) {
	srv := serve(t, fixture(t, "menu_only.html"))
	s := &Service{fetcher: newTestFetcher()}
	if _, err := s.Import(context.Background(), Source{Kind: KindURL, URL: srv.URL}); !errors.Is(err, ErrUnparsable) {
		t.Fatalf("got %v", err)
	}
}

func TestImportTextValidationAndResult(t *testing.T) {
	s := New()
	ctx := context.Background()
	for _, in := range []string{"", "   \n ", strings.Repeat("a", MaxTextBytes+1)} {
		if _, err := s.Import(ctx, Source{Kind: KindText, Text: in}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("len %d: expected ErrInvalid, got %v", len(in), err)
		}
	}
	if _, err := s.Import(ctx, Source{Kind: KindText, Text: "Dzień dobry, jak się masz?"}); !errors.Is(err, ErrUnparsable) {
		t.Errorf("chatter must be unparsable, got %v", err)
	}
	d, err := s.Import(ctx, Source{Kind: KindText, Text: "Zupa\nPorcje: 2\nSkładniki\n1 l wody\nPrzygotowanie\nZagotuj."})
	if err != nil || d.Title != "Zupa" || d.Servings != 2 || d.SourceURL != "" {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestImportRejectsUnknownKind(t *testing.T) {
	if _, err := New().Import(context.Background(), Source{Kind: "ftp"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}
