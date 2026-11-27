package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	// The launcher's URL carries a one-time code, which the server swaps for
	// a cookie the browser then sends on its own. That is what the iframe and
	// image loads rely on.
	if rec := serve(authRequest(http.MethodGet, AuthPath+"?c=wrong")); rec.Code != http.StatusForbidden {
		t.Fatalf("auth with wrong code: %d, want 403", rec.Code)
	}
	if rec := serve(authRequest(http.MethodGet, AuthPath+"?t="+testToken)); rec.Code != http.StatusForbidden {
		t.Fatalf("the token must not sign in through the URL: %d, want 403", rec.Code)
	}
	signIn, err := AuthURL("", testToken, 7878)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(signIn, testToken) {
		t.Fatalf("sign-in URL leaks the token: %q", signIn)
	}
	rec := serve(authRequest(http.MethodGet, signIn))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("auth: %d %q, want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
	if rec := serve(authRequest(http.MethodGet, signIn)); rec.Code != http.StatusForbidden {
		t.Fatalf("a sign-in code must work once: second use %d, want 403", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "local_eml_session_7878" || !cookies[0].HttpOnly ||
		cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %+v", cookies)
	}
	if strings.Contains(cookies[0].Value, testToken) {
		t.Fatalf("cookie carries the token: %q", cookies[0].Value)
	}
	withCookie := authRequest(http.MethodGet, "/api/categories")
	withCookie.AddCookie(cookies[0])
	if rec := serve(withCookie); rec.Code != http.StatusOK {
		t.Fatalf("cookie: %d, want 200", rec.Code)
	}
}

func TestSignInCodeIsBoundToPortAndTime(t *testing.T) {
	s := newTestServer(t)
	s.Port = 7878
	r := s.Router()
	get := func(code string) int {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, authRequest(http.MethodGet, AuthPath+"?c="+code))
		return rec.Code
	}

	other, _ := NewSignInCode(testToken, 7879)
	if got := get(other); got != http.StatusForbidden {
		t.Fatalf("code for another port: %d, want 403", got)
	}
	forged, _ := NewSignInCode("someone-else", 7878)
	if got := get(forged); got != http.StatusForbidden {
		t.Fatalf("code from another token: %d, want 403", got)
	}
	old, _ := signedValue(testToken, signInPurpose, 7878, time.Now().Add(-signInCodeTTL-time.Second))
	if got := get(old); got != http.StatusForbidden {
		t.Fatalf("expired code: %d, want 403", got)
	}
	// A session cookie is not a sign-in code, and the other way round.
	session, _ := signedValue(testToken, sessionPurpose, 7878, time.Now())
	if got := get(session); got != http.StatusForbidden {
		t.Fatalf("session value used as a code: %d, want 403", got)
	}
}

func TestSessionCookieExpiresAndRenews(t *testing.T) {
	s := newTestServer(t)
	s.Port = 7878
	r := s.Router()
	withCookie := func(value string) *httptest.ResponseRecorder {
		req := authRequest(http.MethodGet, "/api/categories")
		req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: value})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	at := func(age time.Duration) string {
		v, _ := signedValue(testToken, sessionPurpose, 7878, time.Now().Add(-age))
		return v
	}

	if rec := withCookie(at(time.Hour)); rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("fresh session: %d, cookies %v; want 200 and no renewal", rec.Code, rec.Result().Cookies())
	}
	if rec := withCookie(at(2 * sessionRenewAfter)); rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("day-old session: %d, cookies %v; want 200 and a renewed cookie", rec.Code, rec.Result().Cookies())
	}
	if rec := withCookie(at(sessionTTL + time.Hour)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired session: %d, want 401", rec.Code)
	}
	code, _ := NewSignInCode(testToken, 7878)
	if rec := withCookie(code); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sign-in code used as a cookie: %d, want 401", rec.Code)
	}
	// A cookie from before sign-in codes holds the token; it is swapped for a
	// session rather than rejected.
	rec := withCookie(testToken)
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 || rec.Result().Cookies()[0].Value == testToken {
		t.Fatalf("legacy cookie: %d, cookies %v; want 200 and a new session cookie", rec.Code, rec.Result().Cookies())
	}
}

func TestSignInFormTakesTheTokenInTheBody(t *testing.T) {
	s := newTestServer(t)
	s.Port = 7878
	r := s.Router()
	post := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, AuthPath, strings.NewReader("t="+token))
		req.Host = "127.0.0.1:7878"
		req.Header.Set("Origin", "http://127.0.0.1:7878")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	if rec := post("wrong"); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong token: %d, want 403", rec.Code)
	}
	rec := post(testToken)
	if rec.Code != http.StatusSeeOther || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("form sign-in: %d, cookies %v", rec.Code, rec.Result().Cookies())
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
}
