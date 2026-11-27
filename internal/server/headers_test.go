package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()
	for _, target := range []string{"/", "/healthz", "/api/categories", "/settings/about", "/missing.js"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newLoopbackRequest(http.MethodGet, target, nil))
		h := rec.Header()
		if got := h.Get("Content-Security-Policy"); got != appCSP {
			t.Errorf("%s: CSP = %q", target, got)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "SAMEORIGIN" ||
			h.Get("Referrer-Policy") != "same-origin" || h.Get("Cross-Origin-Resource-Policy") != "same-origin" {
			t.Errorf("%s: headers = %v", target, h)
		}
	}
}

// The app policy must never allow scripts from anywhere but the server.
func TestAppCSPHasNoInlineOrRemoteScripts(t *testing.T) {
	for _, d := range strings.Split(appCSP, "; ") {
		if !strings.HasPrefix(d, "script-src ") && !strings.HasPrefix(d, "default-src ") {
			continue
		}
		for _, bad := range []string{"'unsafe-inline'", "'unsafe-eval'", "http:", "https:", "*", "data:", "blob:"} {
			for _, src := range strings.Fields(d)[1:] {
				if src == bad {
					t.Errorf("%q allows %s", d, bad)
				}
			}
		}
	}
}

// An email body's images come only from its own inline parts, never from the
// rest of the API.
func TestEmailBodyCSPIsScopedToItsOwnParts(t *testing.T) {
	got := csp("127.0.0.1:7878", "abc", false)
	if !strings.Contains(got, "img-src http://127.0.0.1:7878/api/emails/abc/cid/ data:;") || strings.Contains(got, "'self'") {
		t.Fatalf("csp = %q", got)
	}
}
