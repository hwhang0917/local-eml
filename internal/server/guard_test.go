package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// httptest.NewRequest defaults Host to example.com, which loopbackGuard
// rejects. Router-level tests build their requests through here instead.
func newLoopbackRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = "127.0.0.1:7878"
	return req
}

func TestLoopbackGuard(t *testing.T) {
	handler := loopbackGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"loopback ip", "127.0.0.1:7878", "", http.StatusOK},
		{"localhost", "localhost:7878", "", http.StatusOK},
		{"ipv6 loopback", "[::1]:7878", "", http.StatusOK},
		{"same origin post", "127.0.0.1:7878", "http://127.0.0.1:7878", http.StatusOK},
		{"localhost origin", "localhost:7878", "http://localhost:7878", http.StatusOK},
		{"rebound host", "evil.example.com:7878", "", http.StatusForbidden},
		{"cross-site origin", "127.0.0.1:7878", "https://evil.example.com", http.StatusForbidden},
		{"origin lookalike", "127.0.0.1:7878", "http://127.0.0.1.evil.example.com", http.StatusForbidden},
		{"null origin", "127.0.0.1:7878", "null", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/exports/s3", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("host=%q origin=%q: got %d, want %d", tc.host, tc.origin, rec.Code, tc.want)
			}
		})
	}
}
