package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// EnforceTokenScopes refuses a request an API token's scopes do not cover.
//
// It runs as middleware, and that is the point. Checking scopes inside each
// handler means every handler written from now on has to remember; a handler
// added in six months by somebody who never read this file would silently
// accept a token that was supposed to be read-only. One gate that covers
// everything is worth more than a fine-grained model with holes in it.
//
// Only tokens issued by this platform carry scopes. A browser session, an OIDC
// bearer or a token created before scopes existed passes through untouched:
// this middleware can refuse a request, never allow one that would otherwise
// have been refused.
func (h Handlers) EnforceTokenScopes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented, _ := bearerTokenFromHeader(r)
		if presented == "" || !strings.HasPrefix(presented, tokenPrefix+"_") {
			next.ServeHTTP(w, r)
			return
		}
		identity, ok := h.identityFromAPIToken(presented)
		if !ok || len(identity.Scopes) == 0 {
			// Not our token, or an unrestricted one. Authentication itself is
			// the handlers' business; this gate only narrows.
			next.ServeHTTP(w, r)
			return
		}
		if apitoken.Permits(identity.Scopes, r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Naming the scope it would have needed: a pipeline that reads
		// "forbidden" has to guess, and its author guesses "give it every
		// right", which is the outcome scopes exist to prevent.
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":         "this token's scopes do not allow " + r.Method + " " + r.URL.Path,
			"requiredScope": apitoken.Explain(r.Method, r.URL.Path),
			"tokenScopes":   strings.Join(identity.Scopes, ","),
		})
	})
}
