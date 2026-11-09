// Package spa serves the embedded single-page app with a history-API fallback.
package spa

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// file is one embedded file held in memory, with a gzip copy when that is smaller.
type file struct {
	data, gz []byte
	ctype    string
	etag     string
}

// compressible lists the text types worth gzipping; fonts and images are already compressed.
var compressible = map[string]bool{".js": true, ".css": true, ".html": true, ".svg": true, ".json": true, ".md": true, ".txt": true, ".webmanifest": true}

func load(fsys fs.FS) (map[string]*file, error) {
	files := map[string]*file{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		ext := path.Ext(p)
		sum := sha256.Sum256(data)
		f := &file{data: data, ctype: mime.TypeByExtension(ext), etag: `"` + hex.EncodeToString(sum[:8]) + `"`}
		if f.ctype == "" {
			f.ctype = http.DetectContentType(data)
		}
		if compressible[ext] && len(data) >= 1024 {
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			_, _ = zw.Write(data)
			_ = zw.Close()
			if buf.Len() < len(data) {
				f.gz = buf.Bytes()
			}
		}
		files[p] = f
		return nil
	})
	return files, err
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, q, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(enc), "gzip") && strings.ReplaceAll(strings.TrimSpace(q), " ", "") != "q=0" {
			return true
		}
	}
	return false
}

func serve(w http.ResponseWriter, r *http.Request, f *file) {
	h := w.Header()
	h.Set("Content-Type", f.ctype)
	body, etag := f.data, f.etag
	if f.gz != nil {
		h.Add("Vary", "Accept-Encoding")
		if acceptsGzip(r) {
			body, etag = f.gz, strings.TrimSuffix(f.etag, `"`)+`-gz"`
			h.Set("Content-Encoding", "gzip")
		}
	}
	h.Set("ETag", etag)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
}

// Handler serves fsys from memory: hashed assets are cached forever, index.html is revalidated
// by ETag, and text files are sent gzipped to clients that accept it. Unknown paths without an
// extension fall back to index.html so client-side routes work on reload.
func Handler(fsys fs.FS) http.Handler {
	files, err := load(fsys)
	if err != nil {
		panic("spa: " + err.Error())
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if f, ok := files[p]; ok {
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			serve(w, r, f)
			return
		}
		if strings.HasPrefix(p, "assets/") || path.Ext(p) != "" {
			http.NotFound(w, r)
			return
		}
		index, ok := files["index.html"]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		serve(w, r, index)
	})
}
