package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
)

type createRepositoryRequest struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	AuthSecretName string `json:"authSecretName"`
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
	req.AuthSecretName = strings.TrimSpace(req.AuthSecretName)
	if req.Name == "" || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url are required"})
		return
	}

	secretValue, ok := h.resolveRepositorySecretValue(w, userID, req.AuthSecretName)
	if !ok {
		return
	}

	if err := validateRepositoryConnectivity(req.URL, secretValue); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repository validation failed: " + err.Error()})
		return
	}

	item := repository.New(userID, req.Name, req.URL, "", req.AuthSecretName)
	if err := h.repositoryStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create repository"})
		return
	}
	writeJSON(w, http.StatusCreated, item)
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

	if err := validateRepositoryConnectivity(item.URL, secretValue); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"repositoryId": item.ID,
			"reachable":    false,
			"error":        err.Error(),
			"checkedAt":    time.Now().UTC(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"repositoryId": item.ID,
		"reachable":    true,
		"checkedAt":    time.Now().UTC(),
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
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "repository attach") {
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
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "repository detach") {
		return
	}
	if err := h.projectResourceStore.DetachRepository(projectID, repositoryID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach repository"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
