package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testSPA() http.Handler {
	return newSPAHandler(fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html><div id=app></div>")},
		"favicon.png":        {Data: []byte("png")},
		"assets/index-ab.js": {Data: []byte(strings.Repeat("console.log('hi');\n", 200))},
	})
}

func get(h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSPAFallbackOnlyForRoutes(t *testing.T) {
	h := testSPA()
	for _, p := range []string{"/", "/settings/about", "/email/abc123"} {
		rec := get(h, p, nil)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "id=app") {
			t.Errorf("%s: want shell, got %d %q", p, rec.Code, rec.Body.String())
		}
	}
	for _, p := range []string{"/robots.txt", "/llms.txt", "/.well-known/ai-catalog.json", "/assets/index-old.js"} {
		if rec := get(h, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", p, rec.Code)
		}
	}
}

// A browser landing on a missing file gets the app (which shows its own
// not-found page) with a 404 status; scripts, images and fetches still get a
// bare 404, so nothing parses HTML as something else.
func TestSPAMissingFileNavigation(t *testing.T) {
	h := securityHeaders(testSPA())
	nav := map[string]string{"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"}
	for _, p := range []string{"/.env::$DATA?raw", "/robots.txt", "/assets/index-old.js", "/wp-login.php"} {
		rec := get(h, p, nav)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "id=app") {
			t.Errorf("%s: want shell with 404, got %d %q", p, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: content-type %q", p, ct)
		}
		if rec.Header().Get("Content-Security-Policy") != appCSP {
			t.Errorf("%s: CSP missing on the not-found page", p)
		}
		if strings.Contains(rec.Body.String(), ".env") || strings.Contains(rec.Body.String(), "wp-login") {
			t.Errorf("%s: path echoed in the body", p)
		}
	}
	for _, accept := range []string{"", "*/*", "image/avif,image/webp,*/*", "text/css,*/*;q=0.1", "application/json"} {
		rec := get(h, "/assets/index-old.js", map[string]string{"Accept": accept})
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "id=app") {
			t.Errorf("Accept %q: want bare 404, got %d %q", accept, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/robots.txt", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "id=app") {
		t.Errorf("POST: want no shell, got %q", rec.Body.String())
	}
}

func TestSPAGzipsAssets(t *testing.T) {
	h := testSPA()
	rec := get(h, "/assets/index-ab.js", map[string]string{"Accept-Encoding": "gzip, deflate, br"})
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("want gzip, got headers %v", rec.Header())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("content-type %q", ct)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(zr)
	if !strings.HasPrefix(string(body), "console.log") {
		t.Errorf("bad body %q", body[:20])
	}

	for _, ae := range []string{"", "identity", "gzip;q=0"} {
		rec := get(h, "/assets/index-ab.js", map[string]string{"Accept-Encoding": ae})
		if rec.Header().Get("Content-Encoding") != "" || !strings.HasPrefix(rec.Body.String(), "console.log") {
			t.Errorf("Accept-Encoding %q: want plain body", ae)
		}
	}
}

func TestSPAServesPrecompressedWasm(t *testing.T) {
	wasm := []byte("\x00asm\x01\x00\x00\x00" + strings.Repeat("x", 100))
	var gz strings.Builder
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(wasm)
	_ = zw.Close()
	h := newSPAHandler(fstest.MapFS{
		"index.html":                {Data: []byte("<!doctype html>")},
		"assets/rhwp_bg-ab.wasm.gz": {Data: []byte(gz.String())},
	})

	rec := get(h, "/assets/rhwp_bg-ab.wasm", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "gzip" || rec.Body.String() != gz.String() {
		t.Fatalf("gzip client: code %d headers %v", rec.Code, rec.Header())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/wasm" {
		t.Errorf("content-type %q", ct)
	}

	rec = get(h, "/assets/rhwp_bg-ab.wasm", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != string(wasm) {
		t.Errorf("plain client: code %d, %d bytes", rec.Code, rec.Body.Len())
	}

	if rec := get(h, "/assets/missing.wasm", map[string]string{"Accept-Encoding": "gzip"}); rec.Code != http.StatusNotFound {
		t.Errorf("missing wasm: want 404, got %d", rec.Code)
	}
}
