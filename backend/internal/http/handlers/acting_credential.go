package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// RecordActingCredential resolves what authenticated a request and carries it
// to the audit, so an event can name the credential behind an action.
//
// It decides nothing. Every authorisation check downstream resolves the
// caller for itself, exactly as it did before this middleware existed: this
// one only answers "which key turned", for the record kept of it.
//
// It is a middleware, and the outermost one, for two reasons. The handlers
// that most need naming are the ones that answer 204 and say nothing else, so
// asking each to remember would be asking the next handler written to
// remember too. And the audit of a mutation is emitted by AuditMutations
// after the handler has returned, from the request it was given - a context
// value added further in would already be out of reach.
//
// Only this platform's own tokens and its shared secret are resolved here. A
// browser session and an OIDC bearer are left alone: they cost a round trip
// to the identity provider, and the actor a handler records for them is
// already the person who authenticated.
func (h Handlers) RecordActingCredential(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential, ok := h.actingCredential(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithCredential(r.Context(), credential)))
	})
}

func (h Handlers) actingCredential(r *http.Request) (auth.Credential, bool) {
	if presented, _ := bearerTokenFromHeader(r); strings.HasPrefix(presented, tokenPrefix+"_") {
		identity, ok := h.identityFromAPIToken(presented)
		if !ok {
			// An unusable token names nothing. The request is about to be
			// refused by whatever handles it, and inventing an actor for a
			// credential that did not verify would be the opposite of what
			// this records.
			return auth.Credential{}, false
		}
		kind := "personal-token"
		if identity.Component != "" {
			kind = "component-token"
		}
		return auth.Credential{
			Actor:     identity.AuditActor(),
			Component: identity.Component,
			TokenID:   identity.CredentialID,
			Kind:      kind,
		}, true
	}
	// The shared secret, which is one credential for every component at once.
	// That is precisely what it cannot say, and naming the kind is how the
	// trail shows that an event came from the credential that was retired
	// rather than from a component holding its own.
	if strings.TrimSpace(r.Header.Get(serviceHeader)) != "" {
		return auth.Credential{Actor: auth.ServiceUsername, Kind: "shared-secret"}, true
	}
	return auth.Credential{}, false
}
