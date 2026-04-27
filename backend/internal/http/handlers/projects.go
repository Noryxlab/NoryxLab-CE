package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

type createProjectRequest struct {
	Name string `json:"name"`
}

func (h Handlers) ListProjects(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	// Reconcile runtime workspaces first so recovered project IDs are visible
	// before the UI applies project-based filtering.
	h.syncWorkspacesFromRuntime(userID)

	items, err := h.listProjectsForUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}

	if len(items) == 0 {
		seed := project.New(defaultProjectName(userID))
		if err := h.projectStore.Create(seed); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create default project"})
			return
		}
		h.accessStore.SetRole(seed.ID, userID, access.RoleAdmin)
		items = append(items, seed)
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) listProjectsForUser(userID string) ([]project.Project, error) {
	projects, err := h.projectStore.List()
	if err != nil {
		return nil, err
	}

	filtered := make([]project.Project, 0, len(projects))
	for _, item := range projects {
		if _, ok := h.accessStore.GetRole(item.ID, userID); !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func defaultProjectName(userID string) string {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		return "My First Project"
	}
	return "My First Project - " + trimmed
}

func (h Handlers) CreateProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	p := project.New(req.Name)

	if err := h.projectStore.Create(p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create project"})
		return
	}

	h.accessStore.SetRole(p.ID, userID, access.RoleAdmin)

	writeJSON(w, http.StatusCreated, p)
}

func (h Handlers) DeleteProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}

	exists, err := h.projectExists(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if !h.requireProjectRole(w, projectID, userID, func(role access.Role) bool { return role == access.RoleAdmin }, "project deletion") {
		return
	}

	if err := h.deleteProjectWorkspaces(projectID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to delete project workloads: " + err.Error()})
		return
	}

	if err := h.projectStore.DeleteProject(projectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete project"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) deleteProjectWorkspaces(projectID string) error {
	items, err := h.workspaceStore.List()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ProjectID != projectID {
			continue
		}
		if h.runtime != nil {
			if err := h.runtime.DeleteService(item.ServiceName); err != nil {
				return err
			}
			if err := h.runtime.DeletePod(item.PodName); err != nil {
				return err
			}
		}
		if err := h.workspaceStore.Delete(item.ID); err != nil {
			return err
		}
	}
	return nil
}
