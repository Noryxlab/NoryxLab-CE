package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
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
	workspaceVSCodeFilePath    = "/home/noryx/.noryx-profile/vscode/noryx.code-workspace"
	defaultWorkspaceProfileDir = "/home/noryx/.noryx-profile"
)

type workspaceAttachedRepo struct {
	Name       string
	URL        string
	DefaultRef string
}

type workspaceAttachedDataset struct {
	Name   string
	Bucket string
	Prefix string
}

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
			} else if isNotFoundError(err) {
				// Runtime service no longer exists: cleanup stale DB record.
				_ = h.workspaceStore.Delete(item.ID)
				continue
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
		workspaceQuery := url.QueryEscape(workspaceVSCodeFilePath)
		if strings.TrimSpace(accessToken) != "" {
			return fmt.Sprintf("/workspaces/%s/?workspace=%s&token=%s", workspaceID, workspaceQuery, accessToken)
		}
		return fmt.Sprintf("/workspaces/%s/?workspace=%s", workspaceID, workspaceQuery)
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

	attachedRepos, attachedDatasets, err := h.resolveProjectWorkspaceResources(req.ProjectID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve project resources"})
		return
	}
	datasourceEnv, err := h.resolveProjectDatasourceEnv(req.ProjectID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve project datasources"})
		return
	}

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

		bootstrapScript := workspaceBootstrapScript(
			req.IDE,
			record.ID,
			accessToken,
			h.shouldSeedFirstProjectExamples(req.ProjectID, userID),
			profileMountPath,
			projectMountPath,
			attachedRepos,
			attachedDatasets,
			h.minioEndpoint,
			h.minioAccessKey,
			h.minioSecretKey,
			h.minioUseSSL,
		)
		workspaceArgs = []string{bootstrapScript}

		envVars := []noryxruntime.EnvVar{
			{Name: "HOME", Value: "/home/noryx"},
			{Name: "PIP_CACHE_DIR", Value: profileMountPath + "/pip-cache"},
			{Name: "OPENVSCODE_DATA_DIR", Value: profileMountPath + "/vscode"},
			{Name: "JUPYTER_CONFIG_DIR", Value: profileMountPath + "/jupyter/config"},
			{Name: "JUPYTERLAB_SETTINGS_DIR", Value: profileMountPath + "/jupyter/lab/user-settings"},
		}
		envVars = append(envVars, datasourceEnv...)

		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName: podName,
			Image:   record.Image,
			Command: workspaceCommand,
			Args:    workspaceArgs,
			Env:     envVars,
			Ports:                   []int{8888},
			CPURequest:              h.workspaceCPU,
			CPULimit:                h.workspaceCPU,
			MemRequest:              h.workspaceMemory,
			MemLimit:                h.workspaceMemory,
			EphemeralStorageRequest: h.workspaceEphemeralStorageRequest,
			EphemeralStorageLimit:   h.workspaceEphemeralStorageLimit,
			PullSecret:              h.registryPullSecret,
			Volumes:                 volumes,
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

func (h Handlers) resolveProjectWorkspaceResources(projectID, userID string) ([]workspaceAttachedRepo, []workspaceAttachedDataset, error) {
	repoIDs, err := h.projectResourceStore.ListProjectRepositoryIDs(projectID)
	if err != nil {
		return nil, nil, err
	}
	attachedRepos := make([]workspaceAttachedRepo, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		item, found, err := h.repositoryStore.GetByID(repoID)
		if err != nil {
			return nil, nil, err
		}
		if !found || item.OwnerUserID != userID {
			continue
		}
		cloneURL := strings.TrimSpace(item.URL)
		if strings.TrimSpace(item.AuthSecretName) != "" {
			secretValue, err := h.resolveRepositorySecretValueForWorkspace(userID, item.AuthSecretName)
			if err != nil {
				return nil, nil, err
			}
			authURL, err := authenticatedGitURL(cloneURL, secretValue)
			if err != nil {
				return nil, nil, err
			}
			cloneURL = authURL
		}
		attachedRepos = append(attachedRepos, workspaceAttachedRepo{
			Name:       fallbackResourceName(item.Name, item.ID),
			URL:        cloneURL,
			DefaultRef: strings.TrimSpace(item.DefaultRef),
		})
	}

	datasetIDs, err := h.projectResourceStore.ListProjectDatasetIDs(projectID)
	if err != nil {
		return nil, nil, err
	}
	attachedDatasets := make([]workspaceAttachedDataset, 0, len(datasetIDs))
	for _, datasetID := range datasetIDs {
		item, found, err := h.datasetStore.GetByID(datasetID)
		if err != nil {
			return nil, nil, err
		}
		if !found || item.OwnerUserID != userID {
			continue
		}
		attachedDatasets = append(attachedDatasets, workspaceAttachedDataset{
			Name:   fallbackResourceName(item.Name, item.ID),
			Bucket: strings.TrimSpace(item.Bucket),
			Prefix: strings.Trim(strings.TrimSpace(item.Prefix), "/"),
		})
	}

	return attachedRepos, attachedDatasets, nil
}

func (h Handlers) resolveRepositorySecretValueForWorkspace(userID, secretName string) (string, error) {
	item, found, err := h.secretStore.GetByName(userID, strings.TrimSpace(secretName))
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("repository auth secret not found: %s", secretName)
	}
	if strings.TrimSpace(h.secretsMasterKey) == "" {
		return "", fmt.Errorf("secrets encryption key is not configured")
	}
	value, err := security.DecryptString(h.secretsMasterKey, item.ValueEncrypted)
	if err != nil {
		return "", err
	}
	return value, nil
}

func authenticatedGitURL(rawURL, token string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("repository auth currently supports https URLs only")
	}
	u.User = url.UserPassword("oauth2", token)
	return u.String(), nil
}

func fallbackResourceName(name, fallback string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = strings.TrimSpace(fallback)
	}
	return sanitizeWorkspacePathName(trimmed)
}

func sanitizeWorkspacePathName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "resource-" + shortID()
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(strings.ReplaceAll(b.String(), "--", "-"), "-.")
	if out == "" {
		return "resource-" + shortID()
	}
	return out
}

func shellQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "'\"'\"'") + "'"
}

func workspaceBootstrapScript(
	kind string,
	workspaceID string,
	accessToken string,
	seedFirstProjectExamples bool,
	profileMountPath,
	projectMountPath string,
	attachedRepos []workspaceAttachedRepo,
	attachedDatasets []workspaceAttachedDataset,
	minioEndpoint,
	minioAccessKey,
	minioSecretKey string,
	minioUseSSL bool,
) string {
	lines := []string{
		"set -e",
		fmt.Sprintf("mkdir -p %s %s %s", projectMountPath, workspaceReposPath, workspaceDatasetsPath),
		"echo NORYX_WS_BOOTSTRAP_V2 >/tmp/noryx-bootstrap-version || true",
		fmt.Sprintf("echo repos=%d datasets=%d >/tmp/noryx-resource-count || true", len(attachedRepos), len(attachedDatasets)),
		fmt.Sprintf("mkdir -p %s %s %s %s %s",
			profileMountPath,
			profileMountPath+"/vscode",
			profileMountPath+"/jupyter/config",
			profileMountPath+"/jupyter/lab/user-settings",
			profileMountPath+"/pip-cache",
		),
		fmt.Sprintf("if [ -f %s ]; then", workspaceRequirementsFile),
		fmt.Sprintf("  echo '[bootstrap] requirements detected at %s'", workspaceRequirementsFile),
		"  echo '[bootstrap] installing requirements into project venv and user site packages'",
		fmt.Sprintf("  python3 -m venv %s || true", workspaceProjectVenvPath),
		fmt.Sprintf("  if [ -x %s/bin/pip ]; then", workspaceProjectVenvPath),
		fmt.Sprintf("    %s/bin/pip install --disable-pip-version-check -r %s > /tmp/noryx-requirements.log 2>&1 || true", workspaceProjectVenvPath, workspaceRequirementsFile),
		"  fi",
		fmt.Sprintf("  python3 -m pip install --disable-pip-version-check --user -r %s >> /tmp/noryx-requirements.log 2>&1 || true", workspaceRequirementsFile),
		fmt.Sprintf("  if [ -x %s/bin/python ]; then", workspaceProjectVenvPath),
		fmt.Sprintf("    %s/bin/python -m pip list --format=freeze > /tmp/noryx-requirements-installed.txt 2>/dev/null || true", workspaceProjectVenvPath),
		"  fi",
		"  python3 -m pip list --format=freeze > /tmp/noryx-requirements-user-installed.txt 2>/dev/null || true",
		"  echo '[bootstrap] requirements installation completed'",
		"else",
		fmt.Sprintf("  echo '[bootstrap] no requirements file found at %s'", workspaceRequirementsFile),
		"fi",
		// Cleanup legacy symlink shortcuts previously created in /mnt.
		fmt.Sprintf("rm -f %s/home %s/repos %s/datasets || true", projectMountPath, projectMountPath, projectMountPath),
		// Hide filesystem housekeeping entries in IDE explorers.
		fmt.Sprintf("cat > %s <<'EOF'", shellQuote(profileMountPath+"/jupyter/config/jupyter_server_config.py")),
		"c = get_config()",
		"c.ContentsManager.hide_globs = ['__pycache__', '*.pyc', 'lost+found']",
		"EOF",
	}
	if seedFirstProjectExamples {
		lines = append(lines, workspaceSeedExamplesLines(projectMountPath)...)
	}

	for _, repo := range attachedRepos {
		repoDir := workspaceReposPath + "/" + sanitizeWorkspacePathName(repo.Name)
		cloneURL := strings.TrimSpace(repo.URL)
		repoRef := strings.TrimSpace(repo.DefaultRef)
		lines = append(lines,
			fmt.Sprintf("if [ -d %s/.git ]; then", shellQuote(repoDir)),
			fmt.Sprintf("  git -C %s pull --ff-only || true", shellQuote(repoDir)),
			"else",
			fmt.Sprintf("  git clone --depth 1 %s %s || true", shellQuote(cloneURL), shellQuote(repoDir)),
			"fi",
		)
		if repoRef != "" {
			lines = append(lines,
				fmt.Sprintf("if [ -d %s/.git ]; then", shellQuote(repoDir)),
				fmt.Sprintf("  git -C %s fetch --all --tags || true", shellQuote(repoDir)),
				fmt.Sprintf("  git -C %s checkout %s || true", shellQuote(repoDir), shellQuote(repoRef)),
				"fi",
			)
		}
	}

	if len(attachedDatasets) > 0 && strings.TrimSpace(minioEndpoint) != "" && strings.TrimSpace(minioAccessKey) != "" && strings.TrimSpace(minioSecretKey) != "" {
		pythonSecure := "False"
		if minioUseSSL {
			pythonSecure = "True"
		}
		lines = append(lines,
			"python3 - <<'PY' || true",
			"import importlib.util",
			"import os",
			"import site",
			"import subprocess",
			"import sys",
			"",
			"if importlib.util.find_spec('minio') is None:",
			"    subprocess.run([sys.executable, '-m', 'pip', 'install', '--disable-pip-version-check', '--user', 'minio'], check=False)",
			"",
			"user_site = site.getusersitepackages()",
			"if user_site and user_site not in sys.path:",
			"    sys.path.append(user_site)",
			"",
			"try:",
			"    from minio import Minio  # type: ignore",
			"except Exception as exc:",
			"    os.makedirs('/datasets', exist_ok=True)",
			"    with open('/datasets/.noryx_sync_error', 'w', encoding='utf-8') as fh:",
			"        fh.write(str(exc))",
			"    raise SystemExit(0)",
			"",
			fmt.Sprintf("client = Minio(%s, access_key=%s, secret_key=%s, secure=%s)", strconv.Quote(minioEndpoint), strconv.Quote(minioAccessKey), strconv.Quote(minioSecretKey), pythonSecure),
			"datasets = [",
		)
		for _, ds := range attachedDatasets {
			lines = append(lines, fmt.Sprintf(
				"    {'name': %s, 'bucket': %s, 'prefix': %s},",
				strconv.Quote(sanitizeWorkspacePathName(ds.Name)),
				strconv.Quote(strings.TrimSpace(ds.Bucket)),
				strconv.Quote(strings.Trim(strings.TrimSpace(ds.Prefix), "/")),
			))
		}
		lines = append(lines,
			"]",
			"",
			"for ds in datasets:",
			"    target = os.path.join('/datasets', ds['name'])",
			"    os.makedirs(target, exist_ok=True)",
			"    prefix = ds.get('prefix', '').strip('/')",
			"    prefix_with_slash = (prefix + '/') if prefix else ''",
			"    try:",
			"        for obj in client.list_objects(ds['bucket'], prefix=prefix_with_slash, recursive=True):",
			"            object_name = obj.object_name",
			"            rel = object_name[len(prefix_with_slash):] if prefix_with_slash and object_name.startswith(prefix_with_slash) else object_name",
			"            if not rel or rel.endswith('/'):",
			"                continue",
			"            dst = os.path.join(target, rel)",
			"            os.makedirs(os.path.dirname(dst), exist_ok=True)",
			"            client.fget_object(ds['bucket'], object_name, dst)",
			"    except Exception as exc:",
			"        err_file = os.path.join(target, '.noryx_sync_error')",
			"        with open(err_file, 'w', encoding='utf-8') as fh:",
			"            fh.write(str(exc))",
			"",
			"import threading",
			"import time",
			"",
			"state = {}",
			"",
			"def sync_loop():",
			"    while True:",
			"        try:",
			"            for ds in datasets:",
			"                base = os.path.join('/datasets', ds['name'])",
			"                if not os.path.isdir(base):",
			"                    continue",
			"                prefix = ds.get('prefix', '').strip('/')",
			"                for root, _, files in os.walk(base):",
			"                    for fn in files:",
			"                        if fn == '.noryx_sync_error':",
			"                            continue",
			"                        local_path = os.path.join(root, fn)",
			"                        rel = os.path.relpath(local_path, base).replace('\\\\', '/')",
			"                        object_name = ((prefix + '/') if prefix else '') + rel",
			"                        try:",
			"                            mtime = int(os.path.getmtime(local_path))",
			"                            size = int(os.path.getsize(local_path))",
			"                            key = (ds['bucket'], object_name)",
			"                            sig = (mtime, size)",
			"                            if state.get(key) == sig:",
			"                                continue",
			"                            client.fput_object(ds['bucket'], object_name, local_path)",
			"                            state[key] = sig",
			"                        except Exception as exc:",
			"                            err_file = os.path.join(base, '.noryx_sync_error')",
			"                            with open(err_file, 'w', encoding='utf-8') as fh:",
			"                                fh.write(str(exc))",
			"        except Exception:",
			"            pass",
			"        time.sleep(10)",
			"",
			"threading.Thread(target=sync_loop, daemon=True).start()",
			"time.sleep(1)",
			"PY",
		)
	}

	if kind == "vscode" {
		lines = append(lines,
			fmt.Sprintf("cat > %s <<'EOF'", shellQuote(profileMountPath+"/vscode/noryx.code-workspace")),
			"{",
			"  \"folders\": [",
			"    { \"path\": \"/mnt\" },",
			"    { \"path\": \"/repos\" },",
			"    { \"path\": \"/datasets\" },",
			"    { \"path\": \"/home\" }",
			"  ],",
			"  \"settings\": {",
			"    \"files.exclude\": {",
			"      \"**/lost+found\": true",
			"    }",
			"  }",
			"}",
			"EOF",
			fmt.Sprintf("if [ -x %s/bin/python ]; then export PATH=%s/bin:$PATH; fi", workspaceProjectVenvPath, workspaceProjectVenvPath),
			"exec openvscode-server \\",
			"  --host 0.0.0.0 \\",
			"  --port 8888 \\",
			"  --without-connection-token \\",
			fmt.Sprintf("  --server-base-path /workspaces/%s \\", workspaceID),
			fmt.Sprintf("  --default-workspace %s \\", profileMountPath+"/vscode/noryx.code-workspace"),
			"  --telemetry-level off \\",
			fmt.Sprintf("  %s", shellQuote(profileMountPath+"/vscode/noryx.code-workspace")),
		)
		return strings.Join(lines, "\n")
	}

	lines = append(lines,
		fmt.Sprintf("if [ -x %s/bin/python ]; then export PATH=%s/bin:$PATH; fi", workspaceProjectVenvPath, workspaceProjectVenvPath),
		"exec python3 -m jupyterlab \\",
		"  --ip=0.0.0.0 \\",
		"  --port=8888 \\",
		"  --no-browser \\",
		"  --ServerApp.allow_remote_access=True \\",
		"  --ServerApp.trust_xheaders=True \\",
		"  --ServerApp.root_dir=/ \\",
		"  --ServerApp.default_url=/lab/tree/home/noryx \\",
		fmt.Sprintf("  --ServerApp.base_url=/workspaces/%s/ \\", workspaceID),
		fmt.Sprintf("  --ServerApp.token=%s \\", accessToken),
		"  --ServerApp.password=",
	)
	return strings.Join(lines, "\n")
}

func (h Handlers) shouldSeedFirstProjectExamples(projectID, userID string) bool {
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	if projectID == "" || userID == "" {
		return false
	}
	items, err := h.projectStore.List()
	if err != nil {
		return false
	}
	expected := defaultProjectName(userID)
	for _, p := range items {
		if strings.TrimSpace(p.ID) != projectID {
			continue
		}
		return strings.TrimSpace(p.Name) == expected
	}
	return false
}

func workspaceSeedExamplesLines(projectMountPath string) []string {
	seedMarker := projectMountPath + "/.noryx-seed-v1"
	return []string{
		fmt.Sprintf("if [ ! -f %s ]; then", shellQuote(seedMarker)),
		"  echo '[bootstrap] seeding first-project app/api examples'",
		fmt.Sprintf("  mkdir -p %s/examples/app %s/examples/api", shellQuote(projectMountPath), shellQuote(projectMountPath)),
		fmt.Sprintf("  cat > %s <<'EOF'", shellQuote(projectMountPath+"/examples/README.md")),
		"# Noryx First Project Examples",
		"",
		"- App example: `examples/app/app.py`",
		"- API example: `examples/api/main.py`",
		"",
		"Run App from `examples/app/app.py` with your App workload.",
		"Run API from `examples/api/main.py` with FastAPI/Uvicorn.",
		"EOF",
		fmt.Sprintf("  cat > %s <<'EOF'", shellQuote(projectMountPath+"/examples/app/app.py")),
		"from http.server import BaseHTTPRequestHandler, HTTPServer",
		"",
		"HOST = '0.0.0.0'",
		"PORT = 8080",
		"",
		"class Handler(BaseHTTPRequestHandler):",
		"    def do_GET(self):",
		"        body = b'Noryx demo app is running\\n'",
		"        self.send_response(200)",
		"        self.send_header('Content-Type', 'text/plain; charset=utf-8')",
		"        self.send_header('Content-Length', str(len(body)))",
		"        self.end_headers()",
		"        self.wfile.write(body)",
		"",
		"if __name__ == '__main__':",
		"    HTTPServer((HOST, PORT), Handler).serve_forever()",
		"EOF",
		fmt.Sprintf("  cat > %s <<'EOF'", shellQuote(projectMountPath+"/examples/api/main.py")),
		"from fastapi import FastAPI",
		"from pydantic import BaseModel",
		"",
		"app = FastAPI(title='Noryx Demo API')",
		"",
		"class ScoreInput(BaseModel):",
		"    age: int",
		"    bmi: float",
		"",
		"@app.get('/health')",
		"def health():",
		"    return {'status': 'ok'}",
		"",
		"@app.post('/score')",
		"def score(payload: ScoreInput):",
		"    value = round((payload.age * 0.1) + (payload.bmi * 0.9), 2)",
		"    return {'score': value}",
		"EOF",
		fmt.Sprintf("  cat > %s <<'EOF'", shellQuote(projectMountPath+"/examples/api/requirements.txt")),
		"fastapi==0.116.1",
		"uvicorn[standard]==0.35.0",
		"pydantic==2.11.7",
		"EOF",
		fmt.Sprintf("  cat > %s <<'EOF'", shellQuote(projectMountPath+"/examples/api/run.sh")),
		"#!/usr/bin/env sh",
		"set -eu",
		"python3 -m pip install --disable-pip-version-check -r /mnt/examples/api/requirements.txt",
		"python3 -m uvicorn main:app --host 0.0.0.0 --port 8080 --app-dir /mnt/examples/api",
		"EOF",
		fmt.Sprintf("  chmod +x %s", shellQuote(projectMountPath+"/examples/api/run.sh")),
		fmt.Sprintf("  touch %s", shellQuote(seedMarker)),
		"fi",
	}
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
			if !isNotFoundError(err) {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace service delete failed: " + err.Error()})
				return
			}
		}
		if err := h.runtime.DeletePod(record.PodName); err != nil {
			if !isNotFoundError(err) {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace pod delete failed: " + err.Error()})
				return
			}
		}
	}

	if err := h.workspaceStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete workspace"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
