package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

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

func (h Handlers) ListBuilds(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	h.syncBuildsFromRuntime()

	items, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}

	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	filtered := make([]build.Build, 0, len(items))
	for _, item := range items {
		if projectFilter != "" && item.ProjectID != projectFilter {
			continue
		}
		if _, allowed := h.accessStore.GetRole(item.ProjectID, userID); !allowed {
			continue
		}
		filtered = append(filtered, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": filtered})
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

func (h Handlers) syncBuildsFromRuntime() {
	discovery, ok := h.runtime.(noryxruntime.BuildDiscovery)
	if !ok {
		return
	}
	runtimeItems, err := discovery.ListBuilds()
	if err != nil {
		return
	}

	for _, item := range runtimeItems {
		buildID := strings.TrimSpace(item.BuildID)
		projectID := strings.TrimSpace(item.ProjectID)
		if buildID == "" || projectID == "" {
			continue
		}
		h.ensureProjectInStore(projectID)

		existing, found, err := h.buildStore.GetByID(buildID)
		if err != nil {
			continue
		}

		record := build.Build{
			ID:               buildID,
			ProjectID:        projectID,
			GitRepository:    strings.TrimSpace(item.GitRepository),
			GitRef:           strings.TrimSpace(item.GitRef),
			DockerfilePath:   strings.TrimSpace(item.DockerfilePath),
			ContextPath:      strings.TrimSpace(item.ContextPath),
			DestinationImage: strings.TrimSpace(item.DestinationImage),
			JobName:          strings.TrimSpace(item.JobName),
			Status:           strings.TrimSpace(item.Status),
			CreatedAt:        time.Now().UTC(),
		}

		if found {
			record = existing
			record.ProjectID = projectID
			if v := strings.TrimSpace(item.GitRepository); v != "" {
				record.GitRepository = v
			}
			if v := strings.TrimSpace(item.GitRef); v != "" {
				record.GitRef = v
			}
			if v := strings.TrimSpace(item.DockerfilePath); v != "" {
				record.DockerfilePath = v
			}
			if v := strings.TrimSpace(item.ContextPath); v != "" {
				record.ContextPath = v
			}
			if v := strings.TrimSpace(item.DestinationImage); v != "" {
				record.DestinationImage = v
			}
			if v := strings.TrimSpace(item.JobName); v != "" {
				record.JobName = v
			}
			if v := strings.TrimSpace(item.Status); v != "" {
				record.Status = v
			}
			if record.CreatedAt.IsZero() {
				record.CreatedAt = time.Now().UTC()
			}
		} else if record.Status == "" {
			record.Status = "submitted"
		}

		_ = h.buildStore.Upsert(record)
	}
}
