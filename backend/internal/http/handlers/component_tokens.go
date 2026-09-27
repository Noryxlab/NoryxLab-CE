package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// Credentials for the platform's own components.
//
// Every component - the backup runner, the restore rehearsal, the validator -
// authenticated with one shared secret carrying the global administrator role.
// Three consequences an operator only discovers under pressure: it cannot be
// revoked for one component without breaking the others, its use cannot be
// told apart in an audit, and a component that only reads a health endpoint
// holds a credential that can delete a project.
//
// A component token fixes all three. It is the same token machinery as a
// person's, so the scope middleware covers it without a second gate, and it is
// named, revocable alone, and limited to the scopes it was given.

type componentTokenRequest struct {
	Component string   `json:"component"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expiresAt,omitempty"`
}

func (h Handlers) ListComponentTokens(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "component tokens are managed by an administrator"})
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not configured"})
		return
	}
	all, err := h.apiTokenStore.ListAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list component tokens"})
		return
	}
	items := make([]apitoken.Token, 0, len(all))
	for _, token := range all {
		if strings.TrimSpace(token.Component) != "" {
			items = append(items, token)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "scopes": apitoken.AllScopes()})
}

func (h Handlers) CreateComponentToken(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "component tokens are managed by an administrator"})
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not configured"})
		return
	}
	var req componentTokenRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Component = strings.TrimSpace(req.Component)
	if req.Component == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "component is required: a credential nobody can name is a credential nobody revokes"})
		return
	}
	for _, scope := range req.Scopes {
		if !apitoken.ValidScope(scope) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown scope " + scope})
			return
		}
	}
	// An unscoped component token would be exactly the shared secret again, so
	// it has to be asked for rather than obtained by omission.
	scopes := apitoken.NormalizeScopes(req.Scopes)

	var expires *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ExpiresAt))
		if err != nil || !parsed.After(time.Now()) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expiresAt must be an RFC3339 time in the future"})
			return
		}
		expires = &parsed
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = req.Component
	}
	id, secret, err := newTokenParts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate a token"})
		return
	}
	sum := sha256.Sum256([]byte(secret))
	token := apitoken.Token{
		ID: id, Component: req.Component, Name: name,
		Scopes:    scopes,
		CreatedAt: time.Now().UTC(), SecretHash: sum[:],
		ExpiresAt: expires,
	}
	if err := h.apiTokenStore.Put(token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create the component token"})
		return
	}
	token.SecretHash = nil
	h.emitAudit(r, identity.UserID(), "component-token.create", "component-token", token.ID, "", "success", "", map[string]any{
		"component": req.Component, "scopes": scopes,
	})
	// Shown once, like every other issued credential.
	// The whole token, not just its secret half: the caller presents
	// "<prefix>_<id>_<secret>" and nothing else parses. Returning the bare
	// secret produced a credential that authenticated nowhere, and said so
	// only as "invalid bearer token".
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  token,
		"secret": tokenPrefix + "_" + token.ID + "_" + secret,
		"note":   "this secret is shown once and cannot be recovered",
	})
}

// RotateComponentToken issues a replacement and revokes the original.
//
// A secret is stored as a hash and shown once, which is the only arrangement
// where losing the database does not hand somebody every component's
// credential. The cost is that a secret mislaid is a secret gone - and the
// answer to that is not to keep it readable, it is to make replacing it a
// single deliberate act.
//
// Server-side and in this order for a reason: create, then revoke. Composed
// by a client as two calls, a failure between them leaves two live
// credentials for one component - which is exactly how seven backup-runner
// tokens accumulated on the DC, one per redeploy, none revoked. If the
// revocation fails here, the replacement is revoked in turn and the caller is
// told nothing changed, because one working credential is the invariant worth
// protecting.
func (h Handlers) RotateComponentToken(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "component tokens are managed by an administrator"})
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not configured"})
		return
	}
	previousID := strings.TrimSpace(r.PathValue("tokenID"))
	previous, found, err := h.apiTokenStore.Get(previousID)
	if err != nil || !found || strings.TrimSpace(previous.Component) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "component token not found"})
		return
	}
	if previous.RevokedAt != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "this credential is already revoked; create a new one"})
		return
	}

	id, secret, err := newTokenParts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate a token"})
		return
	}
	sum := sha256.Sum256([]byte(secret))
	// Same component, same scopes, same expiry: a rotation replaces the
	// secret and nothing else, so nobody has to remember what the old one
	// could do.
	replacement := apitoken.Token{
		ID: id, Component: previous.Component, Name: previous.Name,
		Scopes:    previous.Scopes,
		CreatedAt: time.Now().UTC(), SecretHash: sum[:],
		ExpiresAt: previous.ExpiresAt,
	}
	if err := h.apiTokenStore.Put(replacement); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create the replacement"})
		return
	}
	if _, err := h.apiTokenStore.Revoke(previousID, previous.UserID, time.Now().UTC()); err != nil {
		// Leave one credential, not two. The replacement goes back rather than
		// staying alive beside the one it failed to retire.
		if _, undo := h.apiTokenStore.Revoke(id, replacement.UserID, time.Now().UTC()); undo != nil {
			log.Printf("component token rotate: replacement %s could not be revoked after a failed rotation: %v", id, undo)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke the previous credential; nothing changed"})
		return
	}
	replacement.SecretHash = nil
	h.emitAudit(r, identity.UserID(), "component-token.rotate", "component-token", id, "", "success", "", map[string]any{
		"component": previous.Component, "replaced": previousID,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  replacement,
		"secret": tokenPrefix + "_" + replacement.ID + "_" + secret,
		"note":   "this secret is shown once and cannot be recovered",
	})
}

func (h Handlers) DeleteComponentToken(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "component tokens are managed by an administrator"})
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not configured"})
		return
	}
	id := strings.TrimSpace(r.PathValue("tokenID"))
	token, found, err := h.apiTokenStore.Get(id)
	if err != nil || !found || strings.TrimSpace(token.Component) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "component token not found"})
		return
	}
	// Stamped rather than deleted, like every other token: a credential that
	// vanishes leaves an auditor unable to say when access ended.
	if _, err := h.apiTokenStore.Revoke(id, token.UserID, time.Now().UTC()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke the component token"})
		return
	}
	h.emitAudit(r, identity.UserID(), "component-token.revoke", "component-token", id, "", "success", "", map[string]any{
		"component": token.Component,
	})
	w.WriteHeader(http.StatusNoContent)
}
