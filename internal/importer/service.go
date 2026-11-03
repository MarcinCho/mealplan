package importer

import (
	"context"
	"strings"
)

type Service struct {
	fetcher *Fetcher
}

func New() *Service { return &Service{fetcher: NewFetcher()} }

// Import turns a link or pasted text into a reviewed-ready Draft. It never saves anything.
func (s *Service) Import(ctx context.Context, src Source) (Draft, error) {
	switch src.Kind {
	case KindText:
		return importText(src.Text)
	case KindURL:
		return s.importURL(ctx, src.URL)
	}
	return Draft{}, invalid("nieznany rodzaj źródła")
}

func importText(text string) (Draft, error) {
	if len(text) > MaxTextBytes {
		return Draft{}, invalid("tekst może mieć najwyżej %d KB", MaxTextBytes>>10)
	}
	if strings.TrimSpace(text) == "" {
		return Draft{}, invalid("tekst przepisu jest pusty")
	}
	d := ParseText(text)
	if len(d.Ingredients)+len(d.Steps) == 0 {
		return Draft{}, ErrUnparsable
	}
	d.finish()
	return d, nil
}

func (s *Service) importURL(ctx context.Context, raw string) (Draft, error) {
	body, err := s.fetcher.Fetch(ctx, raw)
	if err != nil {
		return Draft{}, err
	}
	d, err := ParseHTML(body)
	if err != nil {
		return Draft{}, err
	}
	d.SourceURL = strings.TrimSpace(raw)
	d.finish()
	return d, nil
}
