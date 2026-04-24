package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createWorkspaceRequest struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

func (h Handlers) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	// Keep API/UI idempotent after back restarts: rebuild missing workspace records
	// from running Kubernetes pods labeled as noryx workspaces.
	h.syncWorkspacesFromRuntime()

	items, err := h.workspaceStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list workspaces"})
		return
	}

	filtered := make([]workspace.Workspace, 0, len(items))
	readiness, hasReadiness := h.runtime.(noryxruntime.WorkspaceReadiness)
	for _, item := range items {
		if _, allowed := h.accessStore.GetRole(item.ProjectID, userID); !allowed {
			continue
		}
		if hasReadiness && item.ServiceName != "" {
			ready, err := readiness.IsServiceReady(item.ServiceName)
			if err == nil {
				if ready {
					item.Status = "running"
				} else {
					item.Status = "launching"
				}
			}
		}
		filtered = append(filtered, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": filtered})
}

func (h Handlers) syncWorkspacesFromRuntime() {
	discovery, ok := h.runtime.(noryxruntime.WorkspaceDiscovery)
	if !ok {
		return
	}
	runtimeItems, err := discovery.ListWorkspaces()
	if err != nil {
		return
	}
	for _, item := range runtimeItems {
		if strings.TrimSpace(item.WorkspaceID) == "" || strings.TrimSpace(item.ProjectID) == "" {
			continue
		}
		_, found, err := h.workspaceStore.GetByID(item.WorkspaceID)
		if err != nil || found {
			continue
		}
		record := workspace.Workspace{
			ID:         item.WorkspaceID,
			ProjectID:  item.ProjectID,
			Kind:       "jupyter",
			Name:       item.PodName,
			Image:      item.Image,
			PodName:    item.PodName,
			ServiceName: item.ServiceName,
			CPU:        h.workspaceCPU,
			Memory:     h.workspaceMemory,
			Status:     "running",
			AccessURL:  fmt.Sprintf("/workspaces/%s/tree", item.WorkspaceID),
			CreatedAt:  time.Now().UTC(),
		}
		_ = h.workspaceStore.Create(record)
	}
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

	// Idempotency baseline for V1:
	// - if a workspace with the same name already exists in this project, reuse it
	// - if no name is provided, reuse the first workspace already present in this project
	existing, foundExisting, err := h.findExistingWorkspace(req.ProjectID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check existing workspaces"})
		return
	}
	if foundExisting {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	podName := "ws-" + shortID()
	serviceName := podName
	accessToken := shortID() + shortID()

	record := workspace.NewJupyter(
		req.ProjectID,
		req.Name,
		h.workspaceJupyterImage,
		podName,
		serviceName,
		h.workspaceCPU,
		h.workspaceMemory,
		"",
		accessToken,
	)
	record.AccessURL = fmt.Sprintf("/workspaces/%s/lab", record.ID)

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
					"--ServerApp.allow_remote_access=True",
					"--ServerApp.trust_xheaders=True",
					"--ServerApp.base_url=/workspaces/" + record.ID + "/",
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

func (h Handlers) findExistingWorkspace(projectID, workspaceName string) (workspace.Workspace, bool, error) {
	items, err := h.workspaceStore.List()
	if err != nil {
		return workspace.Workspace{}, false, err
	}

	workspaceName = strings.TrimSpace(workspaceName)
	for _, item := range items {
		if item.ProjectID != projectID || item.Kind != "jupyter" {
			continue
		}
		if workspaceName == "" || strings.EqualFold(strings.TrimSpace(item.Name), workspaceName) {
			return item, true, nil
		}
	}
	return workspace.Workspace{}, false, nil
}

func (h Handlers) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	workspaceID := strings.TrimSpace(r.PathValue("workspaceID"))
	if workspaceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspaceID is required"})
		return
	}

	record, found, err := h.workspaceStore.GetByID(workspaceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read workspace"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workspace not found"})
		return
	}

	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "workspace deletion") {
		return
	}

	if h.runtime != nil {
		if err := h.runtime.DeleteService(record.ServiceName); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace service delete failed: " + err.Error()})
			return
		}
		if err := h.runtime.DeletePod(record.PodName); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace pod delete failed: " + err.Error()})
			return
		}
	}

	if err := h.workspaceStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete workspace"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
