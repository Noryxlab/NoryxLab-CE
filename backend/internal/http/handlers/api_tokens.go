package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// Personal API tokens.
//
// A researcher calling the API from a CI job or a notebook had no honest
// option: the platform authenticates people through Keycloak, and a pipeline
// has no browser. The alternatives people reach for otherwise are worse -
// sharing a password, or copying a short-lived browser token and wondering why
// it stops working an hour later.
//
// A token acts as its owner and can never exceed what that person may do, so
// it needs no permissions of its own. That is the whole reason it is safe to
// hand out: the blast radius of a leak is one account, not the platform.

const (
	tokenPrefix     = "noryx"
	tokenIDBytes    = 8  // 16 hex characters, enough to index without collision
	tokenSecretSize = 24 // 192 bits: this is the entire secret
	tokenTouchEvery = time.Minute
)

type createAPITokenRequest struct {
	Name string `json:"name"`
	// ExpiresInDays is optional. A token that never expires is a credential
	// nobody ever revisits, so the interface offers a default rather than
	// forbidding it.
	ExpiresInDays int `json:"expiresInDays,omitempty"`
}

func (h Handlers) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []apitoken.Token{}})
		return
	}
	tokens, err := h.apiTokenStore.ListByUser(identity.UserID())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list api tokens"})
		return
	}
	if tokens == nil {
		tokens = []apitoken.Token{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tokens})
}

func (h Handlers) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not available"})
		return
	}

	var req createAPITokenRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		// Named on purpose: revoking the right one should not require guessing,
		// and "gitlab-ci" beats "token 3".
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a name is required"})
		return
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > 730 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expiresInDays must be between 0 and 730"})
		return
	}

	id, secret, err := newTokenParts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate a token"})
		return
	}
	sum := sha256.Sum256([]byte(secret))
	token := apitoken.Token{
		ID: id, UserID: identity.UserID(), Name: name,
		CreatedAt: time.Now().UTC(), SecretHash: sum[:],
	}
	if req.ExpiresInDays > 0 {
		expiry := token.CreatedAt.AddDate(0, 0, req.ExpiresInDays)
		token.ExpiresAt = &expiry
	}
	if err := h.apiTokenStore.Put(token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the token"})
		return
	}

	h.emitAudit(r, identity.UserID(), "api_token.created", "api_token", token.ID, "", "success", "",
		map[string]any{"name": name})

	// The secret exists in readable form here and nowhere else, ever again.
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  token,
		"secret": tokenPrefix + "_" + id + "_" + secret,
		"note":   "this token is shown once and is not recoverable; store it now",
	})
}

func (h Handlers) DeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such api token"})
		return
	}
	tokenID := strings.TrimSpace(r.PathValue("tokenID"))
	revoked, err := h.apiTokenStore.Revoke(tokenID, identity.UserID(), time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke the token"})
		return
	}
	if !revoked {
		// Same answer whether the token belongs to somebody else or does not
		// exist: distinguishing them would confirm an identifier to whoever
		// guessed it.
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such api token"})
		return
	}
	h.emitAudit(r, identity.UserID(), "api_token.revoked", "api_token", tokenID, "", "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// identityFromAPIToken resolves a personal token to its owner.
//
// It returns the owner's identity with no roles of its own: the token acts as
// the person, and every check downstream is the one that would run for a
// browser session. A token cannot therefore grant what its owner lacks.
func (h Handlers) identityFromAPIToken(presented string) (auth.Identity, bool) {
	if h.apiTokenStore == nil {
		return auth.Identity{}, false
	}
	id, secret, ok := parseToken(presented)
	if !ok {
		return auth.Identity{}, false
	}
	token, found, err := h.apiTokenStore.Get(id)
	if err != nil || !found {
		return auth.Identity{}, false
	}
	sum := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(sum[:], token.SecretHash) != 1 {
		return auth.Identity{}, false
	}
	now := time.Now().UTC()
	if !token.Active(now) {
		return auth.Identity{}, false
	}
	h.touchAPIToken(token, now)
	return auth.Identity{Username: token.UserID, Roles: map[string]struct{}{}}, true
}

// touchAPIToken records use at most once a minute. Without the interval a busy
// pipeline turns every request into a write to record something its owner reads
// once a week.
func (h Handlers) touchAPIToken(token apitoken.Token, now time.Time) {
	if token.LastUsedAt != nil && now.Sub(*token.LastUsedAt) < tokenTouchEvery {
		return
	}
	if err := h.apiTokenStore.Touch(token.ID, now); err != nil {
		log.Printf("could not record use of api token %s: %v", token.ID, err)
	}
}

func newTokenParts() (id, secret string, err error) {
	idBytes := make([]byte, tokenIDBytes)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", err
	}
	secretBytes := make([]byte, tokenSecretSize)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(idBytes), hex.EncodeToString(secretBytes), nil
}

func parseToken(raw string) (id, secret string, ok bool) {
	parts := strings.Split(strings.TrimSpace(raw), "_")
	if len(parts) != 3 || parts[0] != tokenPrefix {
		return "", "", false
	}
	if len(parts[1]) != tokenIDBytes*2 || len(parts[2]) != tokenSecretSize*2 {
		return "", "", false
	}
	return parts[1], parts[2], true
}
