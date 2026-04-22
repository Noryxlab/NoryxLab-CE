package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createBuildRequest struct {
	ProjectID        string `json:"projectId"`
	GitRepository    string `json:"gitRepository"`
	GitRef           string `json:"gitRef"`
	DockerfilePath   string `json:"dockerfilePath"`
	ContextPath      string `json:"contextPath"`
	DestinationImage string `json:"destinationImage"`
}

func (h Handlers) ListBuilds(w http.ResponseWriter, _ *http.Request) {
	items, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateBuild(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	var req createBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.GitRepository = strings.TrimSpace(req.GitRepository)
	req.DestinationImage = strings.TrimSpace(req.DestinationImage)
	if req.ProjectID == "" || req.GitRepository == "" || req.DestinationImage == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId, gitRepository and destinationImage are required"})
		return
	}
	if req.DockerfilePath == "" {
		req.DockerfilePath = "Dockerfile"
	}

	exists, err := h.projectExists(req.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if !h.requireProjectRole(w, req.ProjectID, userID, access.Role.CanRunBuild, "build submission") {
		return
	}

	jobName := "build-" + shortID()
	record := build.New(
		req.ProjectID,
		req.GitRepository,
		req.GitRef,
		req.DockerfilePath,
		req.ContextPath,
		req.DestinationImage,
		jobName,
	)

	if h.runtime != nil {
		err = h.runtime.CreateBuild(noryxruntime.BuildSpec{
			JobName:            jobName,
			ContextGitURL:      req.GitRepository,
			GitRef:             req.GitRef,
			DockerfilePath:     req.DockerfilePath,
			ContextPath:        req.ContextPath,
			DestinationImage:   req.DestinationImage,
			PullSecret:         h.registryPullSecret,
			RegistrySecretName: h.registryPushSecret,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-build",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/build-id":      record.ID,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes build submission failed: " + err.Error()})
			return
		}
	}

	if err := h.buildStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save build"})
		return
	}

	writeJSON(w, http.StatusCreated, record)
}
