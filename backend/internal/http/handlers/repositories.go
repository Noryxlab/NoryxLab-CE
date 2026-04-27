package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
)

type createRepositoryRequest struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	DefaultRef     string `json:"defaultRef"`
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
	req.DefaultRef = strings.TrimSpace(req.DefaultRef)
	req.AuthSecretName = strings.TrimSpace(req.AuthSecretName)
	if req.Name == "" || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url are required"})
		return
	}
	if req.AuthSecretName != "" {
		_, found, err := h.secretStore.GetByName(userID, req.AuthSecretName)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to validate auth secret"})
			return
		}
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authSecretName not found for current user"})
			return
		}
	}
	item := repository.New(userID, req.Name, req.URL, req.DefaultRef, req.AuthSecretName)
	if item.DefaultRef == "" {
		item.DefaultRef = "main"
	}
	if err := h.repositoryStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create repository"})
		return
	}
	writeJSON(w, http.StatusCreated, item)
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
