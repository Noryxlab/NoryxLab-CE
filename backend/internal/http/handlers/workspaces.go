package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createWorkspaceRequest struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

func (h Handlers) ListWorkspaces(w http.ResponseWriter, _ *http.Request) {
	items, err := h.workspaceStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list workspaces"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	var req createWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ProjectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	if req.Name == "" {
		req.Name = "jupyter-" + shortID()
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

	if !h.requireProjectRole(w, req.ProjectID, userID, access.Role.CanLaunchPod, "workspace launch") {
		return
	}

	podName := "ws-" + shortID()
	serviceName := podName
	accessToken := shortID() + shortID()
	accessURL := fmt.Sprintf("http://%s:8888/lab?token=%s", serviceName, accessToken)

	record := workspace.NewJupyter(
		req.ProjectID,
		req.Name,
		h.workspaceJupyterImage,
		podName,
		serviceName,
		h.workspaceCPU,
		h.workspaceMemory,
		accessURL,
		accessToken,
	)

	if h.runtime != nil {
		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName: podName,
			Image:   h.workspaceJupyterImage,
			Command: []string{"python3", "-m", "jupyterlab"},
			Args: []string{
				"--ip=0.0.0.0",
				"--port=8888",
				"--no-browser",
				"--allow-root",
				"--ServerApp.token=" + accessToken,
				"--ServerApp.password=",
			},
			Ports:      []int{8888},
			CPURequest: h.workspaceCPU,
			CPULimit:   h.workspaceCPU,
			MemRequest: h.workspaceMemory,
			MemLimit:   h.workspaceMemory,
			PullSecret: h.registryPullSecret,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-workspace",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/workspace-id":  record.ID,
				"noryx.io/workspace-pod": podName,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace pod launch failed: " + err.Error()})
			return
		}

		err = h.runtime.CreateService(noryxruntime.ServiceSpec{
			Name: serviceName,
			Selector: map[string]string{
				"noryx.io/workspace-pod": podName,
			},
			Port: 8888,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace service creation failed: " + err.Error()})
			return
		}
	}

	if err := h.workspaceStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save workspace"})
		return
	}

	writeJSON(w, http.StatusCreated, record)
}
