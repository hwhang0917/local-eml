package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The server listens on loopback, but loopback is shared by every user and
// program on the machine, so the address alone proves nothing. Each library
// has a random token, kept in a file only its owner can read.
//
// The token itself never goes into a URL or a cookie. A URL ends up in the
// browser's command line (which other accounts can read with ps) and in its
// history; a cookie for 127.0.0.1 is sent to every other local server the
// browser visits, whatever its port. So two things are derived from the token
// instead, both by HMAC:
//
//   - a sign-in code, which the launcher builds and passes to the browser as
//     AuthPath?c=<code>. It works once, for a couple of minutes, on one port.
//   - a session cookie, which the server hands out in exchange. It expires,
//     and is renewed while the browser keeps using it.
//
// From then on every request rides on the cookie, including the viewer
// iframe, image and attachment loads and the progress stream, none of which
// could carry a header. A command-line client sends the token as a bearer
// header instead. Replacing the token file and restarting signs everyone out.
const (
	tokenBytes = 32
	// AuthPath is the sign-in endpoint. GET AuthPath?c=<code> is the
	// launcher's way in; the sign-in page POSTs the token as a form field, so
	// a pasted token stays out of the address bar and the history.
	AuthPath  = "/auth"
	codeQuery = "c"
	tokenForm = "t"
	// Cookies ignore the port, so the service and a temporary session on the
	// same host must not share a cookie name.
	cookiePrefix = "local_eml_session_"
	bearerPrefix = "Bearer "
	tokenMode    = 0o600

	// signInCodeTTL is how long a sign-in code works. It has to outlast a
	// cold browser start on a slow machine.
	signInCodeTTL = 2 * time.Minute
	// sessionTTL is how long a session cookie lasts without being used.
	// sessionRenewAfter is how old one gets before a request replaces it, so
	// a browser in regular use never has to sign in again.
	sessionTTL        = 30 * 24 * time.Hour
	sessionRenewAfter = 24 * time.Hour

	nonceBytes      = 16
	signInPurpose   = "local-eml sign-in v1"
	sessionPurpose  = "local-eml session v1"
	maxAuthFormSize = 4096
)

// sign is HMAC-SHA256 over the purpose, port and fields, keyed by the token.
// The port is part of every signature: a code or cookie made for one server
// means nothing to another one sharing the token (there is none today) or
// answering on a different port.
func sign(token, purpose string, port int, fields ...string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(purpose + "|" + strconv.Itoa(port) + "|" + strings.Join(fields, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

func newNonce() (string, error) {
	raw := make([]byte, nonceBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// signedValue is "<unix time>.<nonce>.<signature>", the shape of both the
// sign-in code and the session cookie.
func signedValue(token, purpose string, port int, at time.Time) (string, error) {
	nonce, err := newNonce()
	if err != nil {
		return "", err
	}
	ts := strconv.FormatInt(at.Unix(), 10)
	return ts + "." + nonce + "." + sign(token, purpose, port, ts, nonce), nil
}

// verifySigned checks a signedValue and returns its time and nonce.
func verifySigned(token, purpose string, port int, v string) (time.Time, string, bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return time.Time{}, "", false
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", false
	}
	want := sign(token, purpose, port, parts[0], parts[1])
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(want)) != 1 {
		return time.Time{}, "", false
	}
	return time.Unix(ts, 0), parts[1], true
}

// NewSignInCode makes a one-time code that signs a browser in to the server
// on port. Anyone holding the token can make one, which is how the launcher
// signs in to a server that is already running without sending it the token:
// a program that merely squats on the port learns nothing it can use.
func NewSignInCode(token string, port int) (string, error) {
	return signedValue(token, signInPurpose, port, time.Now())
}

// usedCodes remembers the sign-in codes already spent, until they expire.
type usedCodes struct {
	mu   sync.Mutex
	seen map[string]time.Time // nonce -> when to forget it
}

// spend reports whether nonce is being used for the first time.
func (u *usedCodes) spend(nonce string, now, expires time.Time) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	for n, exp := range u.seen {
		if now.After(exp) {
			delete(u.seen, n)
		}
	}
	if _, dup := u.seen[nonce]; dup {
		return false
	}
	if u.seen == nil {
		u.seen = map[string]time.Time{}
	}
	u.seen[nonce] = expires
	return true
}

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
// server at base, listening on port, and redirects to the app.
func AuthURL(base, token string, port int) (string, error) {
	code, err := NewSignInCode(token, port)
	if err != nil {
		return "", err
	}
	return base + AuthPath + "?" + codeQuery + "=" + url.QueryEscape(code), nil
}

func (s *Server) cookieName() string {
	return cookiePrefix + strconv.Itoa(s.Port)
}

func (s *Server) tokenMatches(got string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) == 1
}

// setSessionCookie issues a fresh session cookie.
func (s *Server) setSessionCookie(w http.ResponseWriter) error {
	v, err := signedValue(s.Token, sessionPurpose, s.Port, time.Now())
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName(),
		Value:    v,
		Path:     "/",
		MaxAge:   int(sessionTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

// sessionAge is how long ago a valid session cookie was issued.
func (s *Server) sessionAge(value string) (time.Duration, bool) {
	issued, _, ok := verifySigned(s.Token, sessionPurpose, s.Port, value)
	if !ok {
		return 0, false
	}
	age := time.Since(issued)
	if age < -time.Minute || age > sessionTTL {
		return 0, false
	}
	return age, true
}

// requireToken rejects API requests that carry neither a session cookie nor
// a matching bearer header.
func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(s.cookieName()); err == nil {
			age, ok := s.sessionAge(c.Value)
			// Versions before sign-in codes kept the token itself in the
			// cookie. Whoever holds it is entitled to a session, so swap it
			// for one rather than making every browser sign in again.
			legacy := !ok && s.tokenMatches(c.Value)
			if ok || legacy {
				if legacy || age > sessionRenewAfter {
					_ = s.setSessionCookie(w)
				}
				next.ServeHTTP(w, r)
				return
			}
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

// handleAuth turns the launcher's sign-in code into the session cookie and
// sends the browser to the app.
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	issued, nonce, ok := verifySigned(s.Token, signInPurpose, s.Port, r.URL.Query().Get(codeQuery))
	now := time.Now()
	if !ok || now.Sub(issued) > signInCodeTTL || issued.Sub(now) > time.Minute ||
		!s.codes.spend(nonce, now, issued.Add(signInCodeTTL)) {
		http.Error(w, "this sign-in link has expired or was already used; open Local Eml again", http.StatusForbidden)
		return
	}
	s.finishSignIn(w, r)
}

// handleAuthForm signs in with the token itself, pasted into the sign-in
// page. It arrives in the body of a same-origin POST (loopbackGuard has
// checked the Origin), never in the URL.
func (s *Server) handleAuthForm(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormSize)
	if err := r.ParseForm(); err != nil || !s.tokenMatches(strings.TrimSpace(r.PostForm.Get(tokenForm))) {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	s.finishSignIn(w, r)
}

func (s *Server) finishSignIn(w http.ResponseWriter, r *http.Request) {
	if err := s.setSessionCookie(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
