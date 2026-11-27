package server

import (
	"net/http"
	"strings"
)

// appCSP is the policy for the app's own page and, by default, every other
// response. The app has no injection bug we know of; this is the second line
// for the day one appears, because a script running in this origin can read
// and delete the whole library. Scripts and workers only come from the
// server itself, nothing is sent anywhere else, and no other site can put
// the app in a frame.
//
// Handlers that serve untrusted content (an email body, an inline
// attachment) replace it with a stricter policy of their own.
//
// The details that are not obvious:
//   - 'wasm-unsafe-eval' lets the HWP renderer's WebAssembly module compile.
//     It does not allow eval().
//   - style-src 'unsafe-inline': the UI library positions menus and dialogs
//     with style attributes.
//   - img-src http: https: is for printing a message with "load remote
//     images" on. The print frame is a srcdoc document, which inherits this
//     policy on top of its own, so the allowance has to exist here. Remote
//     images in the viewer are still decided by the email body's own policy.
//   - frame-ancestors 'self': the viewer frames the email body and PDFs from
//     this same origin; nothing else may.
var appCSP = strings.Join([]string{
	"default-src 'self'",
	"script-src 'self' 'wasm-unsafe-eval'",
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data: blob: http: https:",
	"font-src 'self' data:",
	"connect-src 'self'",
	"worker-src 'self'",
	"frame-src 'self'",
	"object-src 'none'",
	"base-uri 'none'",
	"form-action 'self'",
	"frame-ancestors 'self'",
}, "; ")

// securityHeaders sets the headers every response should carry. A handler
// that needs something else overwrites the header before it writes.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", appCSP)
		// Never guess a type: a message or attachment served as text must not
		// be run as a script or a page because of how it starts.
		h.Set("X-Content-Type-Options", "nosniff")
		// For browsers that predate frame-ancestors.
		h.Set("X-Frame-Options", "SAMEORIGIN")
		// Links opened from the app must not tell the other site where they
		// came from. Not "no-referrer": under it the browser sends
		// "Origin: null" on the app's own form posts (sign-in, the selection
		// export), which loopbackGuard rightly refuses.
		h.Set("Referrer-Policy", "same-origin")
		// Another origin, including another program's page on a different
		// loopback port, cannot load our responses as images, scripts or
		// media and learn from whether they loaded.
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
