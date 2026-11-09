package spa

import (
	"compress/gzip"
	"io"
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

func getWith(path string, hdr map[string]string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	bigSite.ServeHTTP(rec, req)
	return rec
}

var bigJS = strings.Repeat("console.log('mealplan');\n", 200)

var bigSite = Handler(fstest.MapFS{
	"index.html":     {Data: []byte("<html>app</html>")},
	"assets/big.js":  {Data: []byte(bigJS)},
	"assets/f.woff2": {Data: []byte(strings.Repeat("x", 4096))},
})

func TestGzipsTextAssetsWhenAccepted(t *testing.T) {
	rec := getWith("/assets/big.js", map[string]string{"Accept-Encoding": "br, gzip"})
	if rec.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("headers %v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(zr)
	if string(got) != bigJS {
		t.Fatal("gzip body does not decode to the file")
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
}

func TestPlainWithoutAcceptEncoding(t *testing.T) {
	for _, ae := range []string{"", "gzip;q=0", "br"} {
		rec := getWith("/assets/big.js", map[string]string{"Accept-Encoding": ae})
		if rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != bigJS {
			t.Fatalf("%q: want plain body, got encoding %q", ae, rec.Header().Get("Content-Encoding"))
		}
	}
}

func TestFontsAreNotRecompressed(t *testing.T) {
	rec := getWith("/assets/f.woff2", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("woff2 must be sent as is")
	}
}

func TestIndexRevalidatesWithETag(t *testing.T) {
	first := getWith("/", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("index has no ETag")
	}
	again := getWith("/plan", map[string]string{"If-None-Match": etag})
	if again.Code != 304 {
		t.Fatalf("want 304 for unchanged index, got %d", again.Code)
	}
}
