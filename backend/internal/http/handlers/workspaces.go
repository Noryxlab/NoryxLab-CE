package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
)

type createWorkspaceRequest struct {
	ProjectID    string `json:"projectId"`
	Name         string `json:"name"`
	IDE          string `json:"ide"`
	Image        string `json:"image"`
	StorageSize  string `json:"storageSize"`
	HardwareTier string `json:"hardwareTier"`
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
		"rstudio": true,
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
	Name           string
	URL            string
	AuthEnvName    string
	DefaultRef     string
	GitAuthorName  string
	GitAuthorEmail string
}

type workspaceAttachedDataset struct {
	ID        string
	Name      string
	Bucket    string
	Prefix    string
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	ReadOnly  bool
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
		if !h.hasProjectMembership(userID, item.ProjectID) {
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

func (h Handlers) syncWorkspacesFromRuntime(_ string) {
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
		accessURL := workspaceAccessURL(kind, item.WorkspaceID)
		createdAt := item.CreatedAt
		if createdAt.IsZero() && found {
			createdAt = existingRecord.CreatedAt
		}
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}

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
			CreatedAt:    createdAt,
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

func workspaceAccessURL(kind, workspaceID string) string {
	if kind == "vscode" {
		workspaceQuery := url.QueryEscape(workspaceVSCodeFilePath)
		return fmt.Sprintf("/workspaces/%s/?workspace=%s", workspaceID, workspaceQuery)
	}
	if kind == "rstudio" {
		return fmt.Sprintf("/workspaces/%s/", workspaceID)
	}

	return fmt.Sprintf("/workspaces/%s/lab?reset", workspaceID)
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
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()

	var req createWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	req.StorageSize = strings.TrimSpace(req.StorageSize)
	req.HardwareTier = strings.TrimSpace(req.HardwareTier)
	tier, tierFound := h.resolveHardwareTier(req.HardwareTier)
	if !tierFound {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown hardwareTier"})
		return
	}
	rawIDE := strings.ToLower(strings.TrimSpace(req.IDE))
	if rawIDE == "" {
		req.IDE = "vscode"
	} else if !allowedWorkspaceIDEs[rawIDE] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ide must be one of: jupyter, vscode, rstudio"})
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
	workspaceImage := h.workspaceVSCodeImage
	if req.IDE == "vscode" {
		workspaceImage = h.workspaceVSCodeImage
	} else if req.IDE == "jupyter" {
		workspaceImage = h.workspaceJupyterImage
	} else if req.IDE == "rstudio" {
		workspaceImage = h.workspaceRStudioImage
	}
	if req.Image != "" {
		workspaceImage = req.Image
	}
	if !h.workspaceEnvironmentAllowed(req.ProjectID, workspaceImage, req.IDE) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected environment is not accessible or compatible with " + req.IDE})
		return
	}
	workspaceCommand := []string{"/bin/sh", "/var/run/noryx/bootstrap/bootstrap.sh"}
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
		tier.CPULimit,
		tier.MemoryLimit,
		"",
		accessToken,
	)
	record.AccessURL = workspaceAccessURL(req.IDE, record.ID)
	record.PVCName = pvcName
	record.PVCClass = h.workspacePVCClass
	record.PVCSize = pvcSize
	record.PVCMountPath = projectMountPath

	attachedRepos, attachedDatasets, err := h.resolveProjectWorkspaceResources(req.ProjectID, identity, true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve project resources"})
		return
	}
	datasourceEnv, err := h.resolveProjectDatasourceEnv(req.ProjectID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve project datasources"})
		return
	}
	userSecretData, err := h.resolveUserSecretEnv(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve user secrets"})
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
		datasetVolumes, err := h.ensureDatasetVolumeMounts(attachedDatasets)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to prepare direct S3 dataset mounts: " + err.Error()})
			return
		}
		volumes = append(volumes, datasetVolumes...)

		bootstrapScript := workspaceBootstrapScript(
			req.IDE,
			record.ID,
			accessToken,
			gitAuthorName(identity),
			gitAuthorEmail(identity),
			h.shouldSeedFirstProjectExamples(req.ProjectID, userID),
			profileMountPath,
			projectMountPath,
			attachedRepos,
			len(attachedDatasets),
		)
		workspaceArgs = nil
		bootstrapSecretName := podName + "-bootstrap"
		err = h.runtime.CreateSecret(noryxruntime.SecretSpec{
			Name: bootstrapSecretName,
			Data: map[string]string{"bootstrap.sh": bootstrapScript},
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-workspace-bootstrap",
				"noryx.io/workspace-id":  record.ID,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace bootstrap secret create failed: " + err.Error()})
			return
		}
		userSecretName := podName + "-user-secrets"
		if len(userSecretData) > 0 {
			err = h.runtime.CreateSecret(noryxruntime.SecretSpec{
				Name: userSecretName,
				Data: userSecretData,
				Labels: map[string]string{
					"app.kubernetes.io/name": "noryx-workload-user-secrets",
					"noryx.io/workspace-id":  record.ID,
				},
			})
			if err != nil {
				_ = h.runtime.DeleteSecret(bootstrapSecretName)
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace user secret create failed: " + err.Error()})
				return
			}
		}

		envVars := []noryxruntime.EnvVar{
			{Name: "HOME", Value: "/home/noryx"},
			{Name: "PIP_CACHE_DIR", Value: profileMountPath + "/pip-cache"},
			{Name: "OPENVSCODE_DATA_DIR", Value: profileMountPath + "/vscode"},
			{Name: "JUPYTER_CONFIG_DIR", Value: profileMountPath + "/jupyter/config"},
			{Name: "JUPYTERLAB_SETTINGS_DIR", Value: profileMountPath + "/jupyter/lab/user-settings"},
		}
		envVars = append(envVars, datasourceEnv...)
		envVars = append(envVars, secretEnvRefs(userSecretName, userSecretData)...)

		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName:                 podName,
			Image:                   record.Image,
			Command:                 workspaceCommand,
			Args:                    workspaceArgs,
			Env:                     envVars,
			Ports:                   []int{8888},
			ReadinessPort:           8888,
			CPURequest:              tier.CPURequest,
			CPULimit:                tier.CPULimit,
			MemRequest:              tier.MemoryRequest,
			MemLimit:                tier.MemoryLimit,
			EphemeralStorageRequest: tier.EphemeralRequest,
			EphemeralStorageLimit:   tier.EphemeralStorageLimit,
			PullSecret:              h.registryPullSecret,
			Volumes:                 volumes,
			Secrets: []noryxruntime.SecretMount{{
				SecretName: bootstrapSecretName,
				MountPath:  "/var/run/noryx/bootstrap",
				ReadOnly:   true,
			}},
			Labels: map[string]string{
				"app.kubernetes.io/name":  "noryx-workspace",
				"noryx.io/project-id":     req.ProjectID,
				"noryx.io/workspace-id":   record.ID,
				"noryx.io/workspace-pod":  podName,
				"noryx.io/workspace-kind": req.IDE,
				"noryx.io/hardware-tier":  tier.ID,
			},
		})
		if err != nil {
			_ = h.runtime.DeleteSecret(bootstrapSecretName)
			_ = h.runtime.DeleteSecret(userSecretName)
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
			_ = h.runtime.DeleteSecret(bootstrapSecretName)
			_ = h.runtime.DeleteSecret(userSecretName)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace service creation failed: " + err.Error()})
			return
		}
	}

	if err := h.workspaceStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save workspace"})
		return
	}

	h.emitAudit(r, userID, "workspace.launch", "workspace", record.ID, record.ProjectID, "success", "", map[string]any{
		"name":         record.Name,
		"ide":          record.Kind,
		"podName":      record.PodName,
		"serviceName":  record.ServiceName,
		"image":        record.Image,
		"hardwareTier": tier.ID,
	})
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

func (h Handlers) resolveProjectWorkspaceResources(projectID string, identity auth.Identity, includeExternalDatasets bool) ([]workspaceAttachedRepo, []workspaceAttachedDataset, error) {
	userID := identity.UserID()
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
		attachedRepos = append(attachedRepos, workspaceAttachedRepo{
			Name:           fallbackResourceName(item.Name, item.ID),
			URL:            strings.TrimSpace(item.URL),
			AuthEnvName:    repositoryAuthEnvName(item.AuthSecretName),
			DefaultRef:     strings.TrimSpace(item.DefaultRef),
			GitAuthorName:  strings.TrimSpace(item.GitAuthorName),
			GitAuthorEmail: strings.TrimSpace(item.GitAuthorEmail),
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
		if !found || !h.canReadDataset(item, identity) {
			continue
		}
		if item.Provider != "minio" && !includeExternalDatasets {
			continue
		}
		attached := workspaceAttachedDataset{
			ID:       item.ID,
			Name:     fallbackResourceName(item.Name, item.ID),
			Bucket:   strings.TrimSpace(item.Bucket),
			Prefix:   strings.Trim(strings.TrimSpace(item.Prefix), "/"),
			ReadOnly: !h.canWriteDataset(item, identity),
		}
		if item.Provider == "minio" {
			attached.Endpoint, err = nodeReachableKubernetesServiceEndpoint(h.minioEndpoint, net.LookupHost)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve local dataset mount endpoint: %w", err)
			}
			attached.AccessKey = h.minioAccessKey
			attached.SecretKey = h.minioSecretKey
			attached.UseSSL = h.minioUseSSL
		} else {
			credentialUserID := item.CredentialUserID
			if credentialUserID == "" {
				credentialUserID = item.OwnerUserID
			}
			credentialItem, found, err := h.secretStore.GetByName(credentialUserID, item.CredentialName)
			if err != nil {
				return nil, nil, err
			}
			if !found {
				return nil, nil, fmt.Errorf("dataset S3 credentials not found: %s", item.Name)
			}
			decrypted, err := security.DecryptString(h.secretsMasterKey, credentialItem.ValueEncrypted)
			if err != nil {
				return nil, nil, err
			}
			var credential datasetS3Credential
			if err := json.Unmarshal([]byte(decrypted), &credential); err != nil {
				return nil, nil, err
			}
			endpoint, err := url.Parse(item.Endpoint)
			if err != nil || endpoint.Host == "" {
				return nil, nil, fmt.Errorf("invalid dataset endpoint: %s", item.Name)
			}
			attached.Endpoint = endpoint.Host
			attached.AccessKey = credential.AccessKey
			attached.SecretKey = credential.SecretKey
			attached.UseSSL = strings.EqualFold(endpoint.Scheme, "https")
		}
		attachedDatasets = append(attachedDatasets, attached)
	}

	return attachedRepos, attachedDatasets, nil
}

func nodeReachableKubernetesServiceEndpoint(endpoint string, lookupHost func(string) ([]string, error)) (string, error) {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return "", nil
	}
	scheme := ""
	hostPort := raw
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		scheme = parsed.Scheme + "://"
		hostPort = parsed.Host
	}
	parsed, err := url.Parse("//" + hostPort)
	if err != nil || parsed.Hostname() == "" {
		return "", fmt.Errorf("invalid endpoint %q", endpoint)
	}
	hostname := parsed.Hostname()
	if !strings.Contains(hostname, ".svc.") && !strings.HasSuffix(hostname, ".svc") {
		return raw, nil
	}
	addresses, err := lookupHost(hostname)
	if err != nil || len(addresses) == 0 {
		if err == nil {
			err = fmt.Errorf("no address returned")
		}
		return "", fmt.Errorf("resolve %s: %w", hostname, err)
	}
	resolved := addresses[0]
	if parsed.Port() != "" {
		resolved = net.JoinHostPort(resolved, parsed.Port())
	}
	return scheme + resolved, nil
}

func (h Handlers) ensureDatasetVolumeMounts(attachedDatasets []workspaceAttachedDataset) ([]noryxruntime.PersistentVolumeClaimMount, error) {
	mounts := make([]noryxruntime.PersistentVolumeClaimMount, 0, len(attachedDatasets))
	for _, item := range attachedDatasets {
		volumeName := "dataset-" + sanitizeK8sName(item.ID)
		endpoint := strings.TrimSpace(item.Endpoint)
		if !strings.Contains(endpoint, "://") {
			scheme := "http://"
			if item.UseSSL {
				scheme = "https://"
			}
			endpoint = scheme + endpoint
		}
		if err := h.runtime.EnsureS3Volume(noryxruntime.S3VolumeSpec{
			Name:      volumeName,
			Bucket:    item.Bucket,
			Prefix:    item.Prefix,
			Endpoint:  endpoint,
			AccessKey: item.AccessKey,
			SecretKey: item.SecretKey,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-dataset-volume",
				"noryx.io/dataset-id":    item.ID,
			},
		}); err != nil {
			return nil, err
		}
		mounts = append(mounts, noryxruntime.PersistentVolumeClaimMount{
			ClaimName: volumeName,
			MountPath: workspaceDatasetsPath + "/" + sanitizeWorkspacePathName(item.Name),
			ReadOnly:  item.ReadOnly,
		})
	}
	return mounts, nil
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

func repositoryAuthEnvName(secretName string) string {
	if strings.TrimSpace(secretName) == "" {
		return ""
	}
	return userSecretEnvName(secretName)
}

func repositoryBootstrapLines(repo workspaceAttachedRepo, repoDir string) []string {
	clonePrefix := ""
	if repo.AuthEnvName != "" {
		askPass := "/tmp/noryx-git-askpass-" + sanitizeWorkspacePathName(repo.Name)
		clonePrefix = fmt.Sprintf("GIT_ASKPASS=%s GIT_TERMINAL_PROMPT=0 ", shellQuote(askPass))
		return []string{
			fmt.Sprintf("cat > %s <<'EOF'", shellQuote(askPass)),
			"#!/bin/sh",
			"case \"$1\" in",
			"  *Username*) printf '%s\\n' oauth2 ;;",
			fmt.Sprintf("  *) printf '%%s\\n' \"$%s\" ;;", repo.AuthEnvName),
			"esac",
			"EOF",
			fmt.Sprintf("chmod 700 %s", shellQuote(askPass)),
			fmt.Sprintf("if [ -d %s/.git ]; then", shellQuote(repoDir)),
			fmt.Sprintf("  %sgit -C %s pull --ff-only || true", clonePrefix, shellQuote(repoDir)),
			"else",
			fmt.Sprintf("  %sgit clone --depth 1 %s %s || true", clonePrefix, shellQuote(strings.TrimSpace(repo.URL)), shellQuote(repoDir)),
			"fi",
		}
	}
	return []string{
		fmt.Sprintf("if [ -d %s/.git ]; then", shellQuote(repoDir)),
		fmt.Sprintf("  git -C %s pull --ff-only || true", shellQuote(repoDir)),
		"else",
		fmt.Sprintf("  git clone --depth 1 %s %s || true", shellQuote(strings.TrimSpace(repo.URL)), shellQuote(repoDir)),
		"fi",
	}
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
	gitUserName string,
	gitUserEmail string,
	seedFirstProjectExamples bool,
	profileMountPath,
	projectMountPath string,
	attachedRepos []workspaceAttachedRepo,
	datasetMountCount int,
) string {
	lines := []string{
		"set -e",
		fmt.Sprintf("mkdir -p %s %s %s", projectMountPath, workspaceReposPath, workspaceDatasetsPath),
		"echo NORYX_WS_BOOTSTRAP_V2 >/tmp/noryx-bootstrap-version || true",
		fmt.Sprintf("echo repos=%d datasets=%d >/tmp/noryx-resource-count || true", len(attachedRepos), datasetMountCount),
		fmt.Sprintf("mkdir -p %s %s %s %s %s",
			profileMountPath,
			profileMountPath+"/vscode",
			profileMountPath+"/jupyter/config",
			profileMountPath+"/jupyter/lab/user-settings",
			profileMountPath+"/pip-cache",
		),
		fmt.Sprintf("git config --file %s user.name %s", shellQuote(profileMountPath+"/gitconfig"), shellQuote(gitUserName)),
		fmt.Sprintf("git config --file %s user.email %s", shellQuote(profileMountPath+"/gitconfig"), shellQuote(gitUserEmail)),
		fmt.Sprintf("ln -sfn %s /home/noryx/.gitconfig", shellQuote(profileMountPath+"/gitconfig")),
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
		repoRef := strings.TrimSpace(repo.DefaultRef)
		lines = append(lines, repositoryBootstrapLines(repo, repoDir)...)
		if repoRef != "" {
			lines = append(lines,
				fmt.Sprintf("if [ -d %s/.git ]; then", shellQuote(repoDir)),
				fmt.Sprintf("  git -C %s fetch --all --tags || true", shellQuote(repoDir)),
				fmt.Sprintf("  git -C %s checkout %s || true", shellQuote(repoDir), shellQuote(repoRef)),
				"fi",
			)
		}
		if repo.GitAuthorName != "" {
			lines = append(lines, fmt.Sprintf("if [ -d %s/.git ]; then git -C %s config user.name %s; fi", shellQuote(repoDir), shellQuote(repoDir), shellQuote(repo.GitAuthorName)))
		}
		if repo.GitAuthorEmail != "" {
			lines = append(lines, fmt.Sprintf("if [ -d %s/.git ]; then git -C %s config user.email %s; fi", shellQuote(repoDir), shellQuote(repoDir), shellQuote(repo.GitAuthorEmail)))
		}
	}

	ideCommandPrefix := "exec "
	ideCommandSuffix := ""

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
			"if [ \"${NORYX_AUTO_UPDATE_IDE:-1}\" = \"1\" ] && command -v noryx-sync-ide-tooling >/dev/null 2>&1; then",
			"  (noryx-sync-ide-tooling >> /tmp/noryx-ide-tooling.log 2>&1 || true) &",
			"fi",
			fmt.Sprintf("if [ -x %s/bin/python ]; then export PATH=%s/bin:$PATH; fi", workspaceProjectVenvPath, workspaceProjectVenvPath),
			ideCommandPrefix+"openvscode-server \\",
			"  --host 0.0.0.0 \\",
			"  --port 8888 \\",
			"  --without-connection-token \\",
			fmt.Sprintf("  --server-base-path /workspaces/%s \\", workspaceID),
			fmt.Sprintf("  --default-workspace %s \\", profileMountPath+"/vscode/noryx.code-workspace"),
			"  --telemetry-level off \\",
			fmt.Sprintf("  %s%s", shellQuote(profileMountPath+"/vscode/noryx.code-workspace"), ideCommandSuffix),
		)
		return strings.Join(lines, "\n")
	}
	if kind == "rstudio" {
		lines = append(lines,
			"mkdir -p /var/lib/rstudio-server /home/noryx/.config/rstudio",
			"rm -f /var/lib/rstudio-server/rstudio-os.sqlite /var/lib/rstudio-server/rstudio-os.sqlite-shm /var/lib/rstudio-server/rstudio-os.sqlite-wal || true",
			"for key_file in secure-cookie-key session-rpc-key; do head -c 32 /dev/urandom > /var/lib/rstudio-server/${key_file}; chown noryx:noryx /var/lib/rstudio-server/${key_file}; chmod 600 /var/lib/rstudio-server/${key_file}; done",
			ideCommandPrefix+"/usr/lib/rstudio-server/bin/rserver \\",
			"  --server-daemonize=0 \\",
			"  --www-address=0.0.0.0 \\",
			"  --www-port=8888 \\",
			"  --auth-none=1 \\",
			"  --server-user=noryx \\",
			"  --server-working-dir=/mnt \\",
			fmt.Sprintf("  --www-root-path=/workspaces/%s \\", workspaceID),
			"  --www-same-site=none \\",
			"  --auth-cookies-force-secure=1 \\",
			"  --www-verify-user-agent=0",
		)
		return strings.Join(lines, "\n")
	}

	lines = append(lines,
		"if [ \"${NORYX_AUTO_UPDATE_IDE:-1}\" = \"1\" ] && command -v noryx-sync-ide-tooling >/dev/null 2>&1; then",
		"  (noryx-sync-ide-tooling >> /tmp/noryx-ide-tooling.log 2>&1 || true) &",
		"fi",
		fmt.Sprintf("if [ -x %s/bin/python ]; then export PATH=%s/bin:$PATH; fi", workspaceProjectVenvPath, workspaceProjectVenvPath),
		ideCommandPrefix+"python3 -m jupyterlab \\",
		"  --ip=0.0.0.0 \\",
		"  --port=8888 \\",
		"  --no-browser \\",
		"  --ServerApp.allow_remote_access=True \\",
		"  --ServerApp.trust_xheaders=True \\",
		"  --ServerApp.root_dir=/ \\",
		"  --ServerApp.default_url=/lab/tree/home/noryx \\",
		fmt.Sprintf("  --ServerApp.base_url=/workspaces/%s/ \\", workspaceID),
		fmt.Sprintf("  --ServerApp.token=%s \\", accessToken),
		"  --ServerApp.password="+ideCommandSuffix,
	)
	return strings.Join(lines, "\n")
}

func gitAuthorName(identity auth.Identity) string {
	name := strings.TrimSpace(identity.Username)
	if name == "" {
		name = strings.TrimSpace(identity.UserID())
	}
	if name == "" {
		return "Noryx User"
	}
	return name
}

func gitAuthorEmail(identity auth.Identity) string {
	email := strings.TrimSpace(identity.Email)
	if email != "" {
		return email
	}
	userID := strings.TrimSpace(identity.UserID())
	if userID == "" {
		userID = "user"
	}
	return sanitizeK8sName(userID) + "@users.noryx.local"
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
		if err := h.runtime.DeleteSecret(record.PodName + "-bootstrap"); err != nil {
			if !isNotFoundError(err) {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace bootstrap secret delete failed: " + err.Error()})
				return
			}
		}
		if err := h.runtime.DeleteSecret(record.PodName + "-user-secrets"); err != nil && !isNotFoundError(err) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes workspace user secret delete failed: " + err.Error()})
			return
		}
	}

	if err := h.workspaceStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete workspace"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.emitAudit(r, userID, "workspace.delete", "workspace", record.ID, record.ProjectID, "success", "", map[string]any{
		"name":    record.Name,
		"podName": record.PodName,
	})
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
