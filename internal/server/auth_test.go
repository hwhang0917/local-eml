package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func authRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "127.0.0.1:7878"
	return req
}

func TestAPIRequiresSessionToken(t *testing.T) {
	s := newTestServer(t)
	s.Port = 7878
	r := s.Router()
	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := serve(authRequest(http.MethodGet, "/api/categories")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d, want 401", rec.Code)
	}
	if rec := serve(authRequest(http.MethodGet, "/healthz")); rec.Code != http.StatusOK {
		t.Fatalf("healthz must stay open for the launcher probe: %d", rec.Code)
	}

	bad := authRequest(http.MethodGet, "/api/categories")
	bad.Header.Set("Authorization", bearerPrefix+"wrong")
	if rec := serve(bad); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong bearer: %d, want 401", rec.Code)
	}
	good := authRequest(http.MethodGet, "/api/categories")
	good.Header.Set("Authorization", bearerPrefix+testToken)
	if rec := serve(good); rec.Code != http.StatusOK {
		t.Fatalf("bearer: %d, want 200", rec.Code)
	}

	// The launcher's URL turns the token into a cookie the browser then sends
	// on its own, which is what the iframe and image loads rely on.
	if rec := serve(authRequest(http.MethodGet, AuthPath+"?t=wrong")); rec.Code != http.StatusForbidden {
		t.Fatalf("auth with wrong token: %d, want 403", rec.Code)
	}
	rec := serve(authRequest(http.MethodGet, AuthURL("", testToken)))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("auth: %d %q, want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "local_eml_session_7878" || !cookies[0].HttpOnly ||
		cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %+v", cookies)
	}
	withCookie := authRequest(http.MethodGet, "/api/categories")
	withCookie.AddCookie(cookies[0])
	if rec := serve(withCookie); rec.Code != http.StatusOK {
		t.Fatalf("cookie: %d, want 200", rec.Code)
	}
}

func TestLoadOrCreateTokenPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	a, err := LoadOrCreateToken(path)
	if err != nil || len(a) != tokenBytes*2 {
		t.Fatalf("first: %q %v", a, err)
	}
	b, err := LoadOrCreateToken(path)
	if err != nil || b != a {
		t.Fatalf("second: %q %v, want %q", b, err, a)
	}
	if !strings.Contains(AuthURL("http://127.0.0.1:1", a), AuthPath+"?t="+a) {
		t.Fatalf("AuthURL = %q", AuthURL("http://127.0.0.1:1", a))
	}
}
