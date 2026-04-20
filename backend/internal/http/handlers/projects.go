package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

type createProjectRequest struct {
    Name string `json:"name"`
}

func (h Handlers) ListProjects(w http.ResponseWriter, _ *http.Request) {
    projects, err := h.projectStore.List()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
        return
    }

    writeJSON(w, http.StatusOK, map[string]any{"items": projects})
}

func (h Handlers) CreateProject(w http.ResponseWriter, r *http.Request) {
    var req createProjectRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
        return
    }

    if req.Name == "" {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
        return
    }

    p := project.New(req.Name)

    if err := h.projectStore.Create(p); err != nil {
        writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create project"})
        return
    }

    writeJSON(w, http.StatusCreated, p)
}
