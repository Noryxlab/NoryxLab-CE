package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createWorkspaceRequest struct {
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	IDE         string `json:"ide"`
	StorageSize string `json:"storageSize"`
}

var (
	workspaceStorageSizePattern = regexp.MustCompile(`^([1-9][0-9]*)([A-Za-z]+)$`)
	workspaceStorageUnits       = map[string]string{
		"mi": "Mi",
		"gi": "Gi",
		"ti": "Ti",
		"m":  "M",
		"g":  "G",
		"t":  "T",
	}
	allowedWorkspaceIDEs = map[string]bool{
		"jupyter": true,
		"vscode":  true,
	}
)

const (
	workspaceProjectMountPath  = "/mnt"
	workspaceRequirementsFile  = "/mnt/requirements.txt"
	workspaceProjectVenvPath   = "/mnt/.venv"
	workspaceReposPath         = "/repos"
	workspaceDatasetsPath      = "/datasets"
	defaultWorkspaceProfileDir = "/home/noryx/.noryx-profile"
)

func (h Handlers) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	// Keep API/UI idempotent after back restarts: rebuild missing workspace records
	// from running Kubernetes pods labeled as noryx workspaces.
	h.syncWorkspacesFromRuntime(userID)

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

func (h Handlers) syncWorkspacesFromRuntime(userID string) {
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
		h.ensureProjectInStore(item.ProjectID)
		// After back restart, in-memory RBAC is empty.
		// Re-grant current user admin on discovered runtime projects.
		if strings.TrimSpace(userID) != "" {
			h.accessStore.SetRole(item.ProjectID, userID, access.RoleAdmin)
		}

		existingRecord, found, err := h.workspaceStore.GetByID(item.WorkspaceID)
		if err != nil {
			continue
		}

		if strings.TrimSpace(item.Kind) == "" && found {
			item.Kind = existingRecord.Kind
		}
		if strings.TrimSpace(item.AccessToken) == "" && found {
			item.AccessToken = existingRecord.AccessToken
		}

		recordName := item.PodName
		if found && strings.TrimSpace(existingRecord.Name) != "" {
			recordName = existingRecord.Name
		}

		if found {
			_ = h.workspaceStore.Delete(item.WorkspaceID)
		}
		kind := normalizeWorkspaceKind(item.Kind)
		accessURL := workspaceAccessURL(kind, item.WorkspaceID, item.AccessToken)

		record := workspace.Workspace{
			ID:           item.WorkspaceID,
			ProjectID:    item.ProjectID,
			Kind:         kind,
			Name:         recordName,
			Image:        item.Image,
			PodName:      item.PodName,
			ServiceName:  item.ServiceName,
			PVCName:      "project-" + sanitizeK8sName(item.ProjectID),
			PVCClass:     h.workspacePVCClass,
			PVCSize:      h.workspacePVCSize,
			PVCMountPath: h.workspacePVCMountPath,
			CPU:          h.workspaceCPU,
			Memory:       h.workspaceMemory,
			Status:       "running",
			AccessURL:    accessURL,
			AccessToken:  item.AccessToken,
			CreatedAt:    time.Now().UTC(),
		}
		_ = h.workspaceStore.Create(record)
	}
}

func normalizeWorkspaceKind(raw string) string {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if allowedWorkspaceIDEs[kind] {
		return kind
	}
	return "jupyter"
}

func workspaceAccessURL(kind, workspaceID, accessToken string) string {
	if kind == "vscode" {
		if strings.TrimSpace(accessToken) != "" {
			return fmt.Sprintf("/workspaces/%s/?folder=%s&token=%s", workspaceID, workspaceProjectMountPath, accessToken)
		}
		return fmt.Sprintf("/workspaces/%s/?folder=%s", workspaceID, workspaceProjectMountPath)
	}

	accessURL := fmt.Sprintf("/workspaces/%s/lab?reset", workspaceID)
	if strings.TrimSpace(accessToken) != "" {
		accessURL = fmt.Sprintf("/workspaces/%s/lab?reset&token=%s", workspaceID, accessToken)
	}
	return accessURL
}

func (h Handlers) ensureProjectInStore(projectID string) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return
	}
	items, err := h.projectStore.List()
	if err != nil {
		return
	}
	for _, item := range items {
		if item.ID == projectID {
			return
		}
	}
	shortID := projectID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	_ = h.projectStore.Create(project.Project{
		ID:        projectID,
		Name:      "Recovered Project " + shortID,
		CreatedAt: time.Now().UTC(),
	})
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
	req.StorageSize = strings.TrimSpace(req.StorageSize)
	rawIDE := strings.ToLower(strings.TrimSpace(req.IDE))
	if rawIDE == "" {
		req.IDE = "jupyter"
	} else if !allowedWorkspaceIDEs[rawIDE] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ide must be one of: jupyter, vscode"})
		return
	} else {
		req.IDE = rawIDE
	}
	if req.ProjectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	if req.Name == "" {
		req.Name = req.IDE + "-" + shortID()
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
	existing, foundExisting, err := h.findExistingWorkspace(req.ProjectID, req.Name, req.IDE)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check existing workspaces"})
		return
	}
	if foundExisting {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	podName := "wks-" + shortID()
	serviceName := podName
	pvcName := "project-" + sanitizeK8sName(req.ProjectID)
	profilePVCName := "profile-" + sanitizeK8sName(userID)
	profileMountPath := strings.TrimSpace(h.workspaceProfilePVCMountPath)
	if profileMountPath == "" {
		profileMountPath = defaultWorkspaceProfileDir
	}
	projectMountPath := workspaceProjectMountPath
	accessToken := shortID() + shortID()
	workspaceImage := h.workspaceJupyterImage
	if req.IDE == "vscode" {
		workspaceImage = h.workspaceVSCodeImage
	}
	workspaceCommand := []string{"/bin/sh", "-lc"}
	workspaceArgs := []string{}
	pvcSize := h.workspacePVCSize
	if h.workspacePVCEnabled {
		normalizedDefaultSize, err := normalizeWorkspaceStorageSize(h.workspacePVCSize)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid workspace PVC default size in server config"})
			return
		}
		pvcSize = normalizedDefaultSize

		if req.StorageSize != "" {
			normalizedSize, err := normalizeWorkspaceStorageSize(req.StorageSize)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			pvcSize = normalizedSize
		}
	}

	record := workspace.New(
		req.IDE,
		req.ProjectID,
		req.Name,
		workspaceImage,
		podName,
		serviceName,
		h.workspaceCPU,
		h.workspaceMemory,
		"",
		accessToken,
	)
	record.AccessURL = workspaceAccessURL(req.IDE, record.ID, accessToken)
	record.PVCName = pvcName
	record.PVCClass = h.workspacePVCClass
	record.PVCSize = pvcSize
	record.PVCMountPath = projectMountPath

	if h.runtime != nil {
		if h.workspacePVCEnabled {
			workspaceAccessMode := strings.TrimSpace(h.workspacePVCAccessMode)
			if workspaceAccessMode == "" {
				workspaceAccessMode = "ReadWriteMany"
			}
			err = h.runtime.CreatePersistentVolumeClaim(noryxruntime.PersistentVolumeClaimSpec{
				Name:             pvcName,
				StorageClassName: h.workspacePVCClass,
				Size:             pvcSize,
				AccessModes:      []string{workspaceAccessMode},
				Labels: map[string]string{
					"app.kubernetes.io/name": "noryx-workspace-volume",
					"noryx.io/project-id":    req.ProjectID,
				},
			})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace pvc create failed: " + err.Error()})
				return
			}
		}
		if h.workspaceProfilePVCEnabled {
			profileAccessMode := strings.TrimSpace(h.workspaceProfilePVCAccessMode)
			if profileAccessMode == "" {
				profileAccessMode = "ReadWriteMany"
			}
			err = h.runtime.CreatePersistentVolumeClaim(noryxruntime.PersistentVolumeClaimSpec{
				Name:             profilePVCName,
				StorageClassName: h.workspaceProfilePVCClass,
				Size:             h.workspaceProfilePVCSize,
				AccessModes:      []string{profileAccessMode},
				Labels: map[string]string{
					"app.kubernetes.io/name": "noryx-user-profile",
					"noryx.io/user-id":       sanitizeK8sName(userID),
				},
			})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace profile pvc create failed: " + err.Error()})
				return
			}
		}

		volumes := []noryxruntime.PersistentVolumeClaimMount{}
		if h.workspacePVCEnabled {
			volumes = append(volumes, noryxruntime.PersistentVolumeClaimMount{
				ClaimName: pvcName,
				MountPath: projectMountPath,
			})
		}
		if h.workspaceProfilePVCEnabled {
			volumes = append(volumes, noryxruntime.PersistentVolumeClaimMount{
				ClaimName: profilePVCName,
				MountPath: profileMountPath,
			})
		}

		bootstrapScript := workspaceBootstrapScript(req.IDE, record.ID, accessToken, profileMountPath, projectMountPath)
		workspaceArgs = []string{bootstrapScript}

		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName: podName,
			Image:   record.Image,
			Command: workspaceCommand,
			Args:    workspaceArgs,
			Env: []noryxruntime.EnvVar{
				{Name: "HOME", Value: "/home/noryx"},
				{Name: "PIP_CACHE_DIR", Value: profileMountPath + "/pip-cache"},
				{Name: "OPENVSCODE_DATA_DIR", Value: profileMountPath + "/vscode"},
				{Name: "JUPYTER_CONFIG_DIR", Value: profileMountPath + "/jupyter/config"},
				{Name: "JUPYTERLAB_SETTINGS_DIR", Value: profileMountPath + "/jupyter/lab/user-settings"},
			},
			Ports:      []int{8888},
			CPURequest: h.workspaceCPU,
			CPULimit:   h.workspaceCPU,
			MemRequest: h.workspaceMemory,
			MemLimit:   h.workspaceMemory,
			PullSecret: h.registryPullSecret,
			Volumes:    volumes,
			Labels: map[string]string{
				"app.kubernetes.io/name":  "noryx-workspace",
				"noryx.io/project-id":     req.ProjectID,
				"noryx.io/workspace-id":   record.ID,
				"noryx.io/workspace-pod":  podName,
				"noryx.io/workspace-kind": req.IDE,
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
			_ = h.runtime.DeletePod(podName)
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

func (h Handlers) findExistingWorkspace(projectID, workspaceName, kind string) (workspace.Workspace, bool, error) {
	items, err := h.workspaceStore.List()
	if err != nil {
		return workspace.Workspace{}, false, err
	}

	workspaceName = strings.TrimSpace(workspaceName)
	for _, item := range items {
		if item.ProjectID != projectID {
			continue
		}
		if normalizeWorkspaceKind(item.Kind) != normalizeWorkspaceKind(kind) {
			continue
		}
		if workspaceName == "" || strings.EqualFold(strings.TrimSpace(item.Name), workspaceName) {
			return item, true, nil
		}
	}
	return workspace.Workspace{}, false, nil
}

func normalizeWorkspaceStorageSize(raw string) (string, error) {
	size := strings.TrimSpace(raw)
	matches := workspaceStorageSizePattern.FindStringSubmatch(size)
	if len(matches) != 3 {
		return "", fmt.Errorf("storageSize must match <number><unit>, example: 10Gi or 20G")
	}
	unit, ok := workspaceStorageUnits[strings.ToLower(matches[2])]
	if !ok {
		return "", fmt.Errorf("storageSize unit must be one of: Mi, Gi, Ti, M, G, T")
	}
	return matches[1] + unit, nil
}

func sanitizeK8sName(raw string) string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	if lowered == "" {
		return shortID()
	}
	var b strings.Builder
	lastDash := false
	for _, r := range lowered {
		isAZ09 := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAZ09 {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = shortID()
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return strings.Trim(out, "-")
}

func workspaceBootstrapScript(kind, workspaceID, accessToken, profileMountPath, projectMountPath string) string {
	lines := []string{
		"set -e",
		fmt.Sprintf("mkdir -p %s %s %s", projectMountPath, workspaceReposPath, workspaceDatasetsPath),
		fmt.Sprintf("mkdir -p %s %s %s %s %s",
			profileMountPath,
			profileMountPath+"/vscode",
			profileMountPath+"/jupyter/config",
			profileMountPath+"/jupyter/lab/user-settings",
			profileMountPath+"/pip-cache",
		),
		fmt.Sprintf("if [ -f %s ]; then", workspaceRequirementsFile),
		fmt.Sprintf("  python3 -m venv %s || true", workspaceProjectVenvPath),
		fmt.Sprintf("  if [ -x %s/bin/pip ]; then", workspaceProjectVenvPath),
		fmt.Sprintf("    %s/bin/pip install --disable-pip-version-check -r %s", workspaceProjectVenvPath, workspaceRequirementsFile),
		"  else",
		fmt.Sprintf("    python3 -m pip install --disable-pip-version-check -r %s", workspaceRequirementsFile),
		"  fi",
		"fi",
	}

	if kind == "vscode" {
		lines = append(lines,
			"exec openvscode-server \\",
			"  --host 0.0.0.0 \\",
			"  --port 8888 \\",
			"  --without-connection-token \\",
			fmt.Sprintf("  --server-base-path /workspaces/%s \\", workspaceID),
			fmt.Sprintf("  --default-folder %s \\", projectMountPath),
			"  --telemetry-level off",
		)
		return strings.Join(lines, "\n")
	}

	lines = append(lines,
		"exec python3 -m jupyterlab \\",
		"  --ip=0.0.0.0 \\",
		"  --port=8888 \\",
		"  --no-browser \\",
		"  --ServerApp.allow_remote_access=True \\",
		"  --ServerApp.trust_xheaders=True \\",
		fmt.Sprintf("  --ServerApp.root_dir=%s \\", projectMountPath),
		fmt.Sprintf("  --ServerApp.base_url=/workspaces/%s/ \\", workspaceID),
		fmt.Sprintf("  --ServerApp.token=%s \\", accessToken),
		"  --ServerApp.password=",
	)
	return strings.Join(lines, "\n")
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
