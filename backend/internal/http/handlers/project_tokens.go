package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// Tokens that belong to a project, for callers that are not people.
//
// A personal token dies with its owner. That is tolerable for a developer
// automating their own work and untenable for an application deployed as an
// endpoint: the day its author leaves, the endpoint stops answering and nobody
// connects the two events. A project token outlives whoever created it, is
// revoked by name, and carries the calling scope alone - it reaches the
// project's applications and nothing else on the platform.

type projectTokenRequest struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expiresInDays"`
}

func (h Handlers) ListProjectTokens(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectRole(w, projectID, userID, actionRunBuild, "project tokens") {
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not available"})
		return
	}
	all, err := h.apiTokenStore.ListAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tokens"})
		return
	}
	items := []apitoken.Token{}
	for _, token := range all {
		if strings.TrimSpace(token.ProjectID) == projectID {
			// The hash never leaves the server, even to an administrator.
			token.SecretHash = nil
			items = append(items, token)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateProjectToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectRole(w, projectID, userID, actionRunBuild, "project tokens") {
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not available"})
		return
	}

	var req projectTokenRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a name is required: revoking the right token should not require guessing"})
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
		ID: id, UserID: userID, ProjectID: projectID, Name: name,
		// Invoke and nothing else: the scope is not a choice here, because a
		// credential that leaves the platform to sit in another system must not
		// be able to do anything but call.
		Scopes:    []string{string(apitoken.ScopeInvoke)},
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
	h.emitAudit(r, userID, "project_token.created", "api_token", token.ID, projectID, "success", "",
		map[string]any{"name": name})

	token.SecretHash = nil
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  token,
		"secret": tokenPrefix + "_" + id + "_" + secret,
		"note":   "this token is shown once and is not recoverable; store it now",
	})
}

func (h Handlers) DeleteProjectToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectRole(w, projectID, userID, actionRunBuild, "project tokens") {
		return
	}
	tokenID := strings.TrimSpace(r.PathValue("tokenID"))
	stored, found, err := h.apiTokenStore.Get(tokenID)
	if err != nil || !found || strings.TrimSpace(stored.ProjectID) != projectID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found in this project"})
		return
	}
	// Revoked by its creator's id, which is how the store addresses it - the
	// token belongs to the project, but the row still records who made it.
	if _, err := h.apiTokenStore.Revoke(tokenID, stored.UserID, time.Now().UTC()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke the token"})
		return
	}
	h.emitAudit(r, userID, "project_token.revoked", "api_token", tokenID, projectID, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}
