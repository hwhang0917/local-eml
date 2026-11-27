package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/hwhang0917/local-eml/web"
)

func spaHandler() http.Handler {
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic(err)
	}
	return newSPAHandler(sub)
}

func newSPAHandler(sub fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(sub))
	gz := &gzipCache{fsys: sub}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if strings.HasPrefix(clean, "assets/") && path.Ext(clean) == ".wasm" {
			if servePrecompressed(w, r, sub, clean) {
				return
			}
		}
		if _, err := fs.Stat(sub, clean); err != nil {
			// A missing file (robots.txt, a stale hashed asset after an
			// upgrade, /.well-known/*.json) is a real 404. Handing it the HTML
			// shell makes crawlers and module loaders parse HTML as something
			// else. Only extensionless paths are client-side routes.
			//
			// A browser navigating to such a path (a typo, a probe like
			// /.env) is the exception: it gets the app with a 404 status, and
			// the app's own not-found page, rather than Go's bare text. The
			// page never names the path or says whether anything is there.
			if path.Ext(clean) != "" {
				if wantsHTML(r) {
					serveIndex(w, sub, http.StatusNotFound)
					return
				}
				http.NotFound(w, r)
				return
			}
			serveIndex(w, sub, http.StatusOK)
			return
		}
		switch {
		case strings.HasPrefix(clean, "assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			// Hashed JS/CSS are served from memory gzipped: the main bundle
			// drops from ~340 KB to ~120 KB on the wire.
			if gz.serve(w, r, clean) {
				return
			}
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

// gzipCache compresses embedded text assets once, on first request, and keeps
// the result. The embedded FS never changes while the process runs.
type gzipCache struct {
	fsys fs.FS
	m    sync.Map // name -> []byte
}

// Explicit types: on Windows mime.TypeByExtension reads the registry, which
// can map .js to text/plain and break module scripts.
var gzipTypes = map[string]string{
	".js":   "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".svg":  "image/svg+xml",
	".json": "application/json",
}

func (c *gzipCache) serve(w http.ResponseWriter, r *http.Request, name string) bool {
	ctype, ok := gzipTypes[path.Ext(name)]
	if !ok || r.Header.Get("Range") != "" || !acceptsGzip(r) {
		return false
	}
	body, ok := c.m.Load(name)
	if !ok {
		raw, err := fs.ReadFile(c.fsys, name)
		if err != nil {
			return false
		}
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		body, _ = c.m.LoadOrStore(name, buf.Bytes())
	}
	b := body.([]byte)
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(b)
	}
	return true
}

// servePrecompressed serves a WASM module that the build stored only as
// <name>.gz (the HWP renderer is 28 MB raw, 10 MB gzipped). Browsers always
// accept gzip; anything that doesn't gets it decompressed on the fly.
func servePrecompressed(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name + ".gz")
	if err != nil {
		return false
	}
	defer f.Close()
	h := w.Header()
	h.Set("Content-Type", "application/wasm")
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Add("Vary", "Accept-Encoding")
	if acceptsGzip(r) {
		h.Set("Content-Encoding", "gzip")
		if st, err := f.Stat(); err == nil {
			h.Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		}
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, f)
		}
		return true
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		http.Error(w, "corrupt asset", http.StatusInternalServerError)
		return true
	}
	defer zr.Close()
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, zr)
	}
	return true
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(enc), "gzip") {
			continue
		}
		if q, found := strings.CutPrefix(strings.TrimSpace(params), "q="); found {
			v, err := strconv.ParseFloat(strings.TrimSpace(q), 64)
			return err == nil && v > 0
		}
		return true
	}
	return false
}

// wantsHTML reports whether r is a page load (the address bar, a link)
// rather than a script, stylesheet, image or fetch. Only page loads list
// text/html in Accept.
func wantsHTML(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Accept"), ",") {
		mt, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(mt), "text/html") {
			return true
		}
	}
	return false
}

func serveIndex(w http.ResponseWriter, sub fs.FS, status int) {
	f, err := sub.Open("index.html")
	if err != nil {
		http.Error(w, "index.html missing", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	_, _ = io.Copy(w, f)
}
