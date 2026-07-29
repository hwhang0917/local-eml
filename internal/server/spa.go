package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/hwhang0917/local-eml/web"
)

func spaHandler() http.Handler {
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, err := fs.Stat(sub, clean); err != nil {
			serveIndex(w, sub)
			return
		}
		switch {
		case strings.HasPrefix(clean, "assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case clean == "index.html":
			// Must revalidate, or a cached shell keeps loading old hashed assets
			// after an upgrade.
			w.Header().Set("Cache-Control", "no-cache")
		default:
			// Root-level statics (favicon etc.) aren't content-hashed, so cache
			// briefly rather than immutably.
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, sub fs.FS) {
	f, err := sub.Open("index.html")
	if err != nil {
		http.Error(w, "index.html missing", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.Copy(w, f)
}
