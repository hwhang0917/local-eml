package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Listening on loopback is not an access control: browsers let any page the
// user visits reach 127.0.0.1. Two attacks follow. DNS rebinding — an attacker
// domain re-resolved to 127.0.0.1 becomes same-origin and can read the whole
// library. CSRF — a cross-site POST with a CORS-safelisted content type needs
// no preflight, so /api/exports/s3 ships the archive to an attacker bucket
// while the browser merely hides the response. Pinning Host closes the first,
// pinning Origin the second.
func loopbackGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		// Browsers always send Origin on cross-origin requests; its absence
		// means a non-browser client (curl, the updater), which CSRF can't use.
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !isLoopbackHost(u.Host) {
				http.Error(w, "cross-origin request blocked", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
