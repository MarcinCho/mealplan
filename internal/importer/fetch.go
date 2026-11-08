package importer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	maxPageBytes = 2 << 20
	maxRedirects = 5
	userAgent    = "Mealplan/1.0 (recipe import)"
)

var (
	cgnat   = netip.MustParsePrefix("100.64.0.0/10") // Tailscale and carrier-grade NAT
	thisNet = netip.MustParsePrefix("0.0.0.0/8")

	// Ranges that are never public hosts, or that embed an IPv4 address a translator could route to.
	benchmark    = netip.MustParsePrefix("198.18.0.0/15")
	reservedV4   = netip.MustParsePrefix("240.0.0.0/4") // includes 255.255.255.255
	ietfProtocol = netip.MustParsePrefix("192.0.0.0/24")
	nat64        = netip.MustParsePrefix("64:ff9b::/96")
	nat64Local   = netip.MustParsePrefix("64:ff9b:1::/48")
	sixToFour    = netip.MustParsePrefix("2002::/16")
	v4Compatible = netip.MustParsePrefix("::/96")
	siteLocal    = netip.MustParsePrefix("fec0::/10") // deprecated site-local, still routed on some networks
	teredo       = netip.MustParsePrefix("2001::/32")

	errInvalidRedirect = errors.New("importer: redirect to unsupported address")
)

// blockedAddr reports addresses the server must never connect to on a user's behalf.
func blockedAddr(a netip.Addr) bool {
	a = a.Unmap()
	return !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsMulticast() || a.IsUnspecified() || cgnat.Contains(a) || thisNet.Contains(a) ||
		benchmark.Contains(a) || reservedV4.Contains(a) || ietfProtocol.Contains(a) ||
		nat64.Contains(a) || nat64Local.Contains(a) || sixToFour.Contains(a) || v4Compatible.Contains(a) ||
		siteLocal.Contains(a) || teredo.Contains(a)
}

type Fetcher struct {
	client   *http.Client
	maxBytes int64
	anyPort  bool
}

func NewFetcher() *Fetcher {
	return newFetcher(func(a netip.Addr) bool { return !blockedAddr(a) }, 10*time.Second, false)
}

// newFetcher checks every connection in the dialer after DNS resolution, so redirects and DNS rebinding
// cannot reach an address that allow rejects.
func newFetcher(allow func(netip.Addr) bool, timeout time.Duration, anyPort bool) *Fetcher {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrBlocked
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || !allow(ip) {
				return ErrBlocked
			}
			return nil
		},
	}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	return &Fetcher{
		maxBytes: maxPageBytes,
		anyPort:  anyPort,
		client: &http.Client{
			Transport: tr,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > maxRedirects {
					return ErrFetch
				}
				if err := checkURL(req.URL, anyPort); err != nil {
					return errInvalidRedirect
				}
				return nil
			},
		},
	}
}

func checkURL(u *url.URL, anyPort bool) error {
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return invalid("obsługiwane są tylko adresy http i https")
	case u.Hostname() == "":
		return invalid("adres nie zawiera nazwy serwera")
	case u.User != nil:
		return invalid("adresy z danymi logowania nie są obsługiwane")
	case !anyPort && u.Port() != "" && u.Port() != "80" && u.Port() != "443":
		return invalid("obsługiwane są tylko domyślne porty")
	}
	return nil
}

func fetchErr(err error) error {
	var ne net.Error
	switch {
	case errors.Is(err, ErrBlocked):
		return ErrBlocked
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ErrTimeout
	}
	// The cause is deliberately dropped: dial errors would tell the caller which internal hosts exist.
	return ErrFetch
}

// Fetch downloads a web page and returns it decoded to UTF-8.
func (f *Fetcher) Fetch(ctx context.Context, raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURL {
		return nil, invalid("adres musi mieć od 1 do %d znaków", maxURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, invalid("adres jest nieprawidłowy")
	}
	if err := checkURL(u, f.anyPort); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, invalid("adres jest nieprawidłowy")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	req.Header.Set("Accept-Language", "pl,en;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, errInvalidRedirect) || errors.Is(err, ErrFetch) {
			return nil, ErrFetch
		}
		return nil, fetchErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, ErrFetch
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		if mt, _, err := mime.ParseMediaType(ct); err != nil || (mt != "text/html" && mt != "application/xhtml+xml") {
			return nil, ErrFetch
		}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, fetchErr(err)
	}
	if int64(len(data)) > f.maxBytes {
		return nil, ErrFetch
	}
	r, err := charset.NewReader(bytes.NewReader(data), ct)
	if err != nil {
		return nil, ErrFetch
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, ErrFetch
	}
	return out, nil
}
