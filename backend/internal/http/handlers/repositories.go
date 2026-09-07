package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
)

type createRepositoryRequest struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	DefaultRef     string `json:"defaultRef"`
	AuthSecretName string `json:"authSecretName"`
	AuthType       string `json:"authType"`
	GitAuthorName  string `json:"gitAuthorName"`
	GitAuthorEmail string `json:"gitAuthorEmail"`
}

func (h Handlers) ListRepositories(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.repositoryStore.ListByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list repositories"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req createRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	req.DefaultRef = strings.TrimSpace(req.DefaultRef)
	req.AuthSecretName = strings.TrimSpace(req.AuthSecretName)
	req.GitAuthorName = strings.TrimSpace(req.GitAuthorName)
	req.GitAuthorEmail = strings.TrimSpace(req.GitAuthorEmail)
	if req.Name == "" || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url are required"})
		return
	}
	if err := validateRepositoryGitIdentity(req.GitAuthorName, req.GitAuthorEmail); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	authType, ok := normalizeRepositoryAuthType(w, req.AuthType, req.AuthSecretName)
	if !ok {
		return
	}

	secretValue, ok := h.resolveRepositorySecretValue(w, userID, req.AuthSecretName)
	if !ok {
		return
	}

	if err := refuseUnsupportedTransport(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	scopes, err := checkRepository(req.URL, secretValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repository validation failed: " + err.Error()})
		return
	}

	item := repository.New(userID, req.Name, req.URL, req.DefaultRef, req.AuthSecretName, authType, req.GitAuthorName, req.GitAuthorEmail)
	setRepositoryValidation(&item, nil)
	item.TokenExcessScopes = excessiveTokenScopes(scopes)
	if err := h.repositoryStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create repository"})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) UpdateRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	repositoryID := strings.TrimSpace(r.PathValue("repositoryID"))
	item, found, err := h.repositoryStore.GetByID(repositoryID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read repository"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	var req createRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	req.DefaultRef = strings.TrimSpace(req.DefaultRef)
	req.AuthSecretName = strings.TrimSpace(req.AuthSecretName)
	req.GitAuthorName = strings.TrimSpace(req.GitAuthorName)
	req.GitAuthorEmail = strings.TrimSpace(req.GitAuthorEmail)
	if req.Name == "" || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url are required"})
		return
	}
	if err := validateRepositoryGitIdentity(req.GitAuthorName, req.GitAuthorEmail); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	authType, ok := normalizeRepositoryAuthType(w, req.AuthType, req.AuthSecretName)
	if !ok {
		return
	}
	secretValue, ok := h.resolveRepositorySecretValue(w, userID, req.AuthSecretName)
	if !ok {
		return
	}
	if err := refuseUnsupportedTransport(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updatedScopes, err := checkRepository(req.URL, secretValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repository validation failed: " + err.Error()})
		return
	}
	item.Name = req.Name
	item.URL = req.URL
	item.DefaultRef = req.DefaultRef
	item.AuthSecretName = req.AuthSecretName
	item.AuthType = authType
	item.GitAuthorName = req.GitAuthorName
	item.GitAuthorEmail = req.GitAuthorEmail
	setRepositoryValidation(&item, nil)
	item.TokenExcessScopes = excessiveTokenScopes(updatedScopes)
	item.UpdatedAt = time.Now().UTC()
	if err := h.repositoryStore.Update(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update repository"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func validateRepositoryGitIdentity(name, email string) error {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if (name == "") != (email == "") {
		return fmt.Errorf("gitAuthorName and gitAuthorEmail must be configured together")
	}
	if email == "" {
		return nil
	}
	address, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(strings.TrimSpace(address.Address), email) {
		return fmt.Errorf("gitAuthorEmail must be a valid email address")
	}
	return nil
}

func setRepositoryValidation(item *repository.Repository, validationErr error) {
	now := time.Now().UTC()
	item.LastValidatedAt = &now
	item.Reachable = validationErr == nil
	item.ValidationError = ""
	if validationErr != nil {
		item.ValidationError = validationErr.Error()
	}
}

// normalizeRepositoryAuthType settles what kind of token a repository uses.
//
// The platform records two: a personal access token and a repository one. The
// interface used to send "secret", meaning "a token kept in a secret", and was
// refused with "authType must be persat or prat" - words nobody had ever been
// shown, about a distinction the form never offered. The form offers it now,
// and the plain words are accepted too, because an API client saying "token"
// means the obvious thing.
func normalizeRepositoryAuthType(w http.ResponseWriter, authType, authSecretName string) (string, bool) {
	if strings.TrimSpace(authSecretName) == "" {
		return "none", true
	}
	switch strings.ToLower(strings.TrimSpace(authType)) {
	case "", "secret", "token", "pat", "persat":
		return "persat", true
	case "prat", "repository", "deploy":
		return "prat", true
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "a repository with a secret is authenticated by a personal token (persat) " +
				"or a repository token (prat)",
		})
		return "", false
	}
}

func (h Handlers) ValidateRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	repositoryID := strings.TrimSpace(r.PathValue("repositoryID"))
	if repositoryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repositoryID is required"})
		return
	}
	item, found, err := h.repositoryStore.GetByID(repositoryID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read repository"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}

	secretValue, ok := h.resolveRepositorySecretValue(w, userID, item.AuthSecretName)
	if !ok {
		return
	}

	validationErr := validateRepositoryConnectivity(item.URL, secretValue)
	setRepositoryValidation(&item, validationErr)
	item.UpdatedAt = time.Now().UTC()
	if err := h.repositoryStore.Update(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist repository validation"})
		return
	}
	if validationErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"repositoryId": item.ID,
			"reachable":    false,
			"error":        validationErr.Error(),
			"checkedAt":    item.LastValidatedAt,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"repositoryId": item.ID,
		"reachable":    true,
		"checkedAt":    item.LastValidatedAt,
	})
}

func (h Handlers) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	repositoryID := strings.TrimSpace(r.PathValue("repositoryID"))
	if repositoryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repositoryID is required"})
		return
	}
	item, found, err := h.repositoryStore.GetByID(repositoryID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read repository"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	if err := h.repositoryStore.Delete(repositoryID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete repository"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) resolveRepositorySecretValue(w http.ResponseWriter, userID, secretName string) (string, bool) {
	secretName = strings.TrimSpace(secretName)
	if secretName == "" {
		return "", true
	}
	item, found, err := h.secretStore.GetByName(userID, secretName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to validate auth secret"})
		return "", false
	}
	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authSecretName not found for current user"})
		return "", false
	}
	if item.ExpiresAt != nil && time.Now().UTC().After(*item.ExpiresAt) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authSecretName is expired"})
		return "", false
	}
	if strings.TrimSpace(h.secretsMasterKey) == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "secrets encryption key is not configured"})
		return "", false
	}
	value, err := security.DecryptString(h.secretsMasterKey, item.ValueEncrypted)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to decrypt auth secret"})
		return "", false
	}
	return value, true
}

func (h Handlers) ListProjectRepositories(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectMember(w, projectID, userID, "repository listing") {
		return
	}
	ids, err := h.projectResourceStore.ListProjectRepositoryIDs(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list project repositories"})
		return
	}
	items := make([]repository.Repository, 0, len(ids))
	for _, id := range ids {
		item, found, err := h.repositoryStore.GetByID(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load project repository"})
			return
		}
		if found {
			items = append(items, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AttachProjectRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	repositoryID := strings.TrimSpace(r.PathValue("repositoryID"))
	if projectID == "" || repositoryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and repositoryID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, actionLaunch, "repository attach") {
		return
	}
	item, found, err := h.repositoryStore.GetByID(repositoryID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read repository"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	secretValue, ok := h.resolveRepositorySecretValue(w, userID, item.AuthSecretName)
	if !ok {
		return
	}
	validationErr := validateRepositoryConnectivity(item.URL, secretValue)
	setRepositoryValidation(&item, validationErr)
	item.UpdatedAt = time.Now().UTC()
	if err := h.repositoryStore.Update(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist repository validation"})
		return
	}
	if validationErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repository cannot be attached: " + validationErr.Error()})
		return
	}
	if err := h.projectResourceStore.AttachRepository(projectID, repositoryID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach repository"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) DetachProjectRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	repositoryID := strings.TrimSpace(r.PathValue("repositoryID"))
	if projectID == "" || repositoryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and repositoryID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, actionLaunch, "repository detach") {
		return
	}
	if err := h.projectResourceStore.DetachRepository(projectID, repositoryID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach repository"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// refuseUnsupportedTransport turns away what the platform cannot clone.
//
// The form invited "an HTTPS or SSH address" and the platform has no SSH key
// material at all: no storage, no injection, no known_hosts. An SSH URL was
// accepted, validated against an API that does not care about transport, and
// then failed at clone time inside a workspace - the one place nobody was
// watching. Refusing here says the true thing at the moment somebody can act
// on it.
func refuseUnsupportedTransport(repoURL string) error {
	trimmed := strings.TrimSpace(repoURL)
	if strings.HasPrefix(trimmed, "git@") || strings.HasPrefix(strings.ToLower(trimmed), "ssh://") {
		return errors.New("this platform clones over HTTPS with a token; an SSH address cannot be used yet")
	}
	return nil
}
