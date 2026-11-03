package importer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"mealplan/internal/domain"
)

func allowAll(netip.Addr) bool { return true }

func newTestFetcher() *Fetcher { return newFetcher(allowAll, 2*time.Second, true) }

func TestBlockedAddr(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", "10.0.0.5", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "100.100.100.100", "0.0.0.0", "0.1.2.3",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "fe80::1", "fc00::1", "fd12:3456::1", "ff02::1", "224.0.0.1",
	}
	for _, s := range blocked {
		if !blockedAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "93.184.216.34", "172.32.0.1", "100.128.0.1", "2606:4700::1111"} {
		if blockedAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
}

func TestFetchRejectsBadURLs(t *testing.T) {
	f := NewFetcher()
	for _, u := range []string{"", "ftp://example.com/x", "file:///etc/passwd", "javascript:alert(1)", "http://", "https://user:pw@example.com/", "http://example.com:22/", "http://example.com:8080/", "not a url", "https://" + strings.Repeat("a", 2100) + ".com"} {
		if _, err := f.Fetch(context.Background(), u); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%q: expected ErrInvalid, got %v", u, err)
		}
	}
}

func TestFetchBlocksPrivateTargetsByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer srv.Close()
	f := newFetcher(func(a netip.Addr) bool { return !blockedAddr(a) }, 2*time.Second, true)
	if _, err := f.Fetch(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("loopback server must be blocked, got %v", err)
	}
	for _, u := range []string{"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/latest/meta-data/", "http://10.0.0.1/", "http://100.64.0.1/", "http://localhost/"} {
		if _, err := NewFetcher().Fetch(context.Background(), u); !errors.Is(err, ErrBlocked) {
			t.Errorf("%s: expected ErrBlocked, got %v", u, err)
		}
	}
}

func TestFetchBlocksRedirectToPrivateAddress(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("internal")) }))
	defer target.Close()
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer front.Close()
	dials := 0
	f := newFetcher(func(netip.Addr) bool { dials++; return dials == 1 }, 2*time.Second, true)
	if _, err := f.Fetch(context.Background(), front.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("redirect must be re-checked, got %v", err)
	}
}

func TestFetchFollowsAtMostFiveRedirects(t *testing.T) {
	n := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), srv.URL); !errors.Is(err, ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
	if n > 6 {
		t.Errorf("followed %d requests", n)
	}
}

func TestFetchReadsHTMLAndDecodesLegacyCharsets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-2")
		_, _ = w.Write([]byte("<p>\xb3\xf3d\xbc</p>")) // "łódź" in ISO-8859-2
	}))
	defer srv.Close()
	body, err := newTestFetcher().Fetch(context.Background(), srv.URL)
	if err != nil || !strings.Contains(string(body), "łódź") {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestFetchLimitsAndFailures(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(strings.Repeat("a", 3<<20)))
	}))
	defer big.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), big.URL); !errors.Is(err, ErrFetch) {
		t.Errorf("oversized body: %v", err)
	}

	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), notFound.URL); !errors.Is(err, ErrFetch) {
		t.Errorf("404: %v", err)
	}

	pdf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF"))
	}))
	defer pdf.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), pdf.URL); !errors.Is(err, ErrFetch) {
		t.Errorf("pdf: %v", err)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { time.Sleep(500 * time.Millisecond) }))
	defer slow.Close()
	f := newFetcher(allowAll, 100*time.Millisecond, true)
	if _, err := f.Fetch(context.Background(), slow.URL); !errors.Is(err, ErrTimeout) {
		t.Errorf("slow server: %v", err)
	}
}
