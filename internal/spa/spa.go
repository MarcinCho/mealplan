// Package spa serves the embedded single-page app with a history-API fallback.
package spa

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func Handler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if st, err := fs.Stat(fsys, p); err == nil && !st.IsDir() {
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if p == "index.html" {
				w.Header().Set("Cache-Control", "no-cache")
			}
			if p == "index.html" {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/" // FileServer redirects /index.html to ./
				files.ServeHTTP(w, r2)
				return
			}
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "assets/") || path.Ext(p) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}
