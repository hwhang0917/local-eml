package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// The server listens on loopback, but loopback is shared by every user and
// program on the machine, so the address alone proves nothing. Each library
// has a random token, kept in a file only its owner can read. The launcher
// hands it to the browser once through AuthPath, which turns it into an
// HttpOnly cookie; from then on every request rides on the cookie, including
// the viewer iframe, image and attachment loads and the progress stream, none
// of which could carry a header. A command-line client sends the same token
// as a bearer header instead.
const (
	tokenBytes = 32
	// AuthPath is the sign-in endpoint: GET AuthPath?t=<token> sets the cookie
	// and lands on the app.
	AuthPath   = "/auth"
	tokenQuery = "t"
	// Cookies ignore the port, so the service and a temporary session on the
	// same host must not share a cookie name.
	cookiePrefix = "local_eml_session_"
	cookieMaxAge = 365 * 24 * 60 * 60
	bearerPrefix = "Bearer "
	tokenMode    = 0o600
)

// LoadOrCreateToken reads the library's token from path, minting and saving
// one on first use. The file sits in the owner-only data folder, so reading
// it is the proof that the caller is the same user.
func LoadOrCreateToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return strings.TrimSpace(string(b)), nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(token+"\n"), tokenMode); err != nil {
		return "", err
	}
	return token, nil
}

// AuthURL is the address the launcher opens: it signs the browser in to the
// server at base and redirects to the app.
func AuthURL(base, token string) string {
	return base + AuthPath + "?" + tokenQuery + "=" + url.QueryEscape(token)
}

func (s *Server) cookieName() string {
	return cookiePrefix + strconv.Itoa(s.Port)
}

func (s *Server) tokenMatches(got string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) == 1
}

// requireToken rejects API requests that carry neither the session cookie
// nor a matching bearer header.
func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(s.cookieName()); err == nil && s.tokenMatches(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, bearerPrefix) &&
			s.tokenMatches(strings.TrimPrefix(h, bearerPrefix)) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "unauthorized: open Local Eml from its launcher, or paste the token from `local-eml token`",
			http.StatusUnauthorized)
	})
}

// handleAuth turns the launcher's token into the session cookie and sends the
// browser to the app.
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.tokenMatches(r.URL.Query().Get(tokenQuery)) {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName(),
		Value:    s.Token,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
