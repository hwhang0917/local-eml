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
