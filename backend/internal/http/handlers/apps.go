package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createAppRequest struct {
	ProjectID string   `json:"projectId"`
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Image     string   `json:"image"`
	Command   []string `json:"command"`
	Args      []string `json:"args"`
	Port      int      `json:"port"`
}

var appSlugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,30}[a-z0-9])?$`)

func (h Handlers) ListApps(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.appStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}
	readiness, hasReadiness := h.runtime.(noryxruntime.WorkspaceReadiness)
	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	filtered := make([]app.App, 0, len(items))
	for _, item := range items {
		if projectFilter != "" && item.ProjectID != projectFilter {
			continue
		}
		if _, allowed := h.accessStore.GetRole(item.ProjectID, userID); !allowed {
			continue
		}
		if hasReadiness && strings.TrimSpace(item.ServiceName) != "" {
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

func (h Handlers) CreateApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = normalizeAppSlug(req.Slug)
	req.Image = strings.TrimSpace(req.Image)
	if req.ProjectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	if req.Name == "" {
		req.Name = "app-" + shortID()
	}
	if req.Slug == "" {
		req.Slug = normalizeAppSlug(req.Name)
	}
	if req.Slug == "" || !appSlugPattern.MatchString(req.Slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slug must match [a-z0-9-] and be 2-32 chars"})
		return
	}
	if req.Image == "" {
		req.Image = strings.TrimSpace(h.workspaceJupyterImage)
	}
	if req.Image == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
		return
	}
	if req.Port <= 0 {
		req.Port = 9000
	}
	if req.Port < 1 || req.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "port must be between 1 and 65535"})
		return
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
	if !h.requireProjectRole(w, req.ProjectID, userID, access.Role.CanLaunchPod, "app launch") {
		return
	}
	if existing, found, err := h.appStore.GetBySlug(req.Slug); err == nil && found {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "slug already used by app " + existing.ID})
		return
	}

	podName := "app-" + shortID()
	serviceName := podName
	accessURL := "/apps/" + req.Slug + "/"

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

	command := []string{"/bin/sh", "-lc"}
	userLaunch := strings.TrimSpace(strings.Join(append(req.Command, req.Args...), " "))
	bootstrapScript := appBootstrapScript(req.Port, userLaunch, attachedRepos, attachedDatasets, h.minioEndpoint, h.minioAccessKey, h.minioSecretKey, h.minioUseSSL)
	args := []string{bootstrapScript}

	record := app.New(req.ProjectID, req.Name, req.Slug, req.Image, command, args, req.Port, podName, serviceName, accessURL)

	if h.runtime != nil {
		volumes := []noryxruntime.PersistentVolumeClaimMount{}
		if h.workspacePVCEnabled {
			volumes = append(volumes, noryxruntime.PersistentVolumeClaimMount{
				ClaimName: "project-" + sanitizeK8sName(req.ProjectID),
				MountPath: workspaceProjectMountPath,
			})
		}
		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName:                 podName,
			Image:                   record.Image,
			Command:                 command,
			Args:                    args,
			Env:                     datasourceEnv,
			Ports:                   []int{record.Port},
			CPURequest:              h.workspaceCPU,
			CPULimit:                h.workspaceCPU,
			MemRequest:              h.workspaceMemory,
			MemLimit:                h.workspaceMemory,
			EphemeralStorageRequest: h.workspaceEphemeralStorageRequest,
			EphemeralStorageLimit:   h.workspaceEphemeralStorageLimit,
			PullSecret:              h.registryPullSecret,
			Volumes:                 volumes,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-app",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/app-id":        record.ID,
				"noryx.io/app-slug":      req.Slug,
				"noryx.io/app-pod":       podName,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes app pod launch failed: " + err.Error()})
			return
		}
		err = h.runtime.CreateService(noryxruntime.ServiceSpec{
			Name: serviceName,
			Selector: map[string]string{
				"noryx.io/app-pod": podName,
			},
			Port: record.Port,
		})
		if err != nil {
			_ = h.runtime.DeletePod(podName)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes app service creation failed: " + err.Error()})
			return
		}
	}
	if err := h.appStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save app"})
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (h Handlers) DeleteApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	appID := strings.TrimSpace(r.PathValue("appID"))
	record, found, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "app deletion") {
		return
	}
	if h.runtime != nil {
		_ = h.runtime.DeleteService(record.ServiceName)
		_ = h.runtime.DeletePod(record.PodName)
	}
	if err := h.appStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete app"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeAppSlug(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
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
	if len(out) > 32 {
		out = strings.Trim(out[:32], "-")
	}
	return out
}

func appBootstrapScript(port int, userLaunch string, attachedRepos []workspaceAttachedRepo, attachedDatasets []workspaceAttachedDataset, minioEndpoint, minioAccessKey, minioSecretKey string, minioUseSSL bool) string {
	lines := []string{
		"set -e",
		fmt.Sprintf("mkdir -p %s %s %s", workspaceProjectMountPath, workspaceReposPath, workspaceDatasetsPath),
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
	}
	for _, repo := range attachedRepos {
		repoDir := workspaceReposPath + "/" + sanitizeWorkspacePathName(repo.Name)
		lines = append(lines,
			fmt.Sprintf("if [ -d %s/.git ]; then", shellQuote(repoDir)),
			fmt.Sprintf("  git -C %s pull --ff-only || true", shellQuote(repoDir)),
			"else",
			fmt.Sprintf("  git clone --depth 1 %s %s || true", shellQuote(strings.TrimSpace(repo.URL)), shellQuote(repoDir)),
			"fi",
		)
	}
	if len(attachedDatasets) > 0 && strings.TrimSpace(minioEndpoint) != "" && strings.TrimSpace(minioAccessKey) != "" && strings.TrimSpace(minioSecretKey) != "" {
		lines = append(lines, workspaceDatasetBootstrapLines(attachedDatasets, minioEndpoint, minioAccessKey, minioSecretKey, minioUseSSL)...)
	}
	userLaunch = strings.TrimSpace(userLaunch)
	defaultHTTP := fmt.Sprintf("python3 -m http.server %d --bind 0.0.0.0 --directory /mnt", port)
	lines = append(lines,
		"if [ -n "+shellQuote(userLaunch)+" ]; then",
		"  echo '[bootstrap] using UI command entrypoint'",
		"  "+userLaunch,
		"elif [ -f /mnt/app.sh ]; then",
		"  echo '[bootstrap] using /mnt/app.sh entrypoint'",
		"  chmod +x /mnt/app.sh || true",
		"  /bin/sh /mnt/app.sh",
		"else",
		"  echo '[bootstrap] no /mnt/app.sh found, using default static server'",
		"  "+defaultHTTP,
		"fi",
	)
	return strings.Join(lines, "\n")
}
