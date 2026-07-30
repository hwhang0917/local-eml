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

const attachmentFixture = `From: sender@example.com
Subject: attachments
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="B"

--B
Content-Type: text/plain

see attached
--B
Content-Type: image/png
Content-Disposition: attachment; filename="pixel.png"
Content-Transfer-Encoding: base64

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==
--B
Content-Type: text/html
Content-Disposition: attachment; filename="page.html"

<script>alert(1)</script>
--B--
`

func newAttachmentTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	p := paths.Paths{Base: dir, EML: filepath.Join(dir, "eml"), DB: filepath.Join(dir, "db"),
		Logs: filepath.Join(dir, "logs"), Keys: filepath.Join(dir, "keys")}
	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}
	sha := strings.Repeat("b", 64)
	if err := os.WriteFile(p.BlobFor(sha), []byte(attachmentFixture), 0o600); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	return &Server{Importer: &importer.Importer{Paths: p}}, sha
}

func TestAttachmentInlineOnlyForSafeTypes(t *testing.T) {
	s, sha := newAttachmentTestServer(t)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, path, nil))
		return rec
	}

	// Without inline=1 the image stays a download.
	if d := get("/api/emails/" + sha + "/attachments/0").Header().Get("Content-Disposition"); !strings.HasPrefix(d, "attachment") {
		t.Errorf("default disposition = %q, want attachment", d)
	}

	// inline=1 on a safe type serves inline with the lockdown CSP.
	rec := get("/api/emails/" + sha + "/attachments/0?inline=1")
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "inline") {
		t.Errorf("png inline disposition = %q, want inline", d)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); csp != "default-src 'none'; sandbox" {
		t.Errorf("png CSP = %q", csp)
	}

	// inline=1 on HTML must stay a download — it would execute in our origin.
	rec = get("/api/emails/" + sha + "/attachments/1?inline=1")
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "attachment") {
		t.Errorf("html disposition = %q, want attachment despite inline=1", d)
	}
}

func TestInlineSafe(t *testing.T) {
	for ct, want := range map[string]bool{
		"image/png":                true,
		"image/jpeg; name=\"x\"":   true,
		"image/svg+xml":            false,
		"application/pdf":          true,
		"audio/mpeg":               true,
		"video/mp4":                true,
		"text/plain":               true,
		"text/html":                false,
		"application/octet-stream": false,
	} {
		if got := inlineSafe(ct); got != want {
			t.Errorf("inlineSafe(%q) = %v, want %v", ct, got, want)
		}
	}
}
