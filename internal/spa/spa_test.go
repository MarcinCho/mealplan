package spa

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var site = fstest.MapFS{
	"index.html":     {Data: []byte("<html>app</html>")},
	"assets/app.js":  {Data: []byte("console.log(1)")},
	"assets/app.css": {Data: []byte("body{}")},
}

func get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Handler(site).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := get("/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("index must not be cached: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestServesAssetsWithLongCache(t *testing.T) {
	rec := get("/assets/app.js")
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("assets must be immutable: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestDeepLinksFallBackToIndex(t *testing.T) {
	for _, p := range []string{"/invite/abc123", "/members", "/login"} {
		rec := get(p)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
			t.Errorf("%s: %d %s", p, rec.Code, rec.Body)
		}
	}
}

func TestMissingAssetIsNotFound(t *testing.T) {
	if rec := get("/assets/missing.js"); rec.Code != 404 {
		t.Fatalf("missing asset should be 404, got %d", rec.Code)
	}
}
