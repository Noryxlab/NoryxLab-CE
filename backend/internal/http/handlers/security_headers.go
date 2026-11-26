package handlers

import (
	"net/http"
	"strings"
)

// The headers a browser needs, set by the thing that knows it is a browser.
//
// They used to be set entirely by a Traefik Middleware in the deployment
// overlay. That works exactly as long as an installation runs Traefik and
// applies that overlay, and it is invisible when it stops: nothing fails, no
// log line appears, the headers are simply absent. An installation on a
// managed cluster behind an ALB or an nginx controller would have had none of
// them, and nobody would have found out from the inside.
//
// So the application sets its own. A proxy may still add or override them -
// the ingress is welcome to, and the Traefik overlay still does - but the
// floor no longer depends on somebody else's configuration file.
//
// What is deliberately not here: a Content-Security-Policy. The frontend is a
// single-page application whose policy belongs with the bundle that has to
// satisfy it, and a CSP written here would be guessed rather than derived -
// which ends as `unsafe-inline` and a header that says nothing.
func (h Handlers) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		// A sniffed content type is how an uploaded file becomes a script.
		header.Set("X-Content-Type-Options", "nosniff")
		// The console is never framed. Workspace and application views are
		// served on their own routes by the proxy, not from here.
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// HSTS only when the exchange reaching the browser is TLS. The
		// platform usually sees plain HTTP because the edge terminates, so
		// the forwarded header is the only honest evidence - and claiming
		// HSTS on an installation genuinely served over HTTP would lock
		// users out of an address that has no certificate.
		if requestReachedTheBrowserOverTLS(r) {
			header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func requestReachedTheBrowserOverTLS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Only the first value: a chain of proxies appends, and the one that
	// spoke to the browser is the first.
	forwarded := r.Header.Get("X-Forwarded-Proto")
	if comma := strings.Index(forwarded, ","); comma >= 0 {
		forwarded = forwarded[:comma]
	}
	return strings.EqualFold(strings.TrimSpace(forwarded), "https")
}
