package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createJobRequest struct {
	ProjectID string   `json:"projectId"`
	Name      string   `json:"name"`
	Image     string   `json:"image"`
	Command   []string `json:"command"`
	Args      []string `json:"args"`
}

func (h Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	h.syncJobsFromRuntime()
	items, err := h.jobStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list jobs"})
		return
	}
	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	filtered := make([]job.Job, 0, len(items))
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

func (h Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	if req.ProjectID == "" || req.Image == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId and image are required"})
		return
	}
	if !h.requireProjectRole(w, req.ProjectID, userID, access.Role.CanLaunchPod, "job launch") {
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

	jobName := "job-" + shortID()
	record := job.New(req.ProjectID, req.Name, req.Image, jobName, req.Command, req.Args)

	attachedRepos, attachedDatasets, err := h.resolveProjectWorkspaceResources(req.ProjectID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve project resources"})
		return
	}
	command := req.Command
	args := req.Args
	if len(command) == 0 {
		command = []string{"/bin/sh", "-lc"}
		args = []string{jobBootstrapScript(req.Args, attachedRepos, attachedDatasets, h.minioEndpoint, h.minioAccessKey, h.minioSecretKey, h.minioUseSSL)}
	}

	if h.runtime != nil {
		err = h.runtime.CreateJob(noryxruntime.JobSpec{
			JobName:                 jobName,
			Image:                   req.Image,
			Command:                 command,
			Args:                    args,
			CPURequest:              h.workspaceCPU,
			CPULimit:                h.workspaceCPU,
			MemRequest:              h.workspaceMemory,
			MemLimit:                h.workspaceMemory,
			EphemeralStorageRequest: h.workspaceEphemeralStorageRequest,
			EphemeralStorageLimit:   h.workspaceEphemeralStorageLimit,
			PullSecret:              h.registryPullSecret,
			Volumes: []noryxruntime.PersistentVolumeClaimMount{
				{ClaimName: "project-" + sanitizeK8sName(req.ProjectID), MountPath: workspaceProjectMountPath},
			},
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-job",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/job-id":        record.ID,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes job launch failed: " + err.Error()})
			return
		}
	}
	if err := h.jobStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save job"})
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (h Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	jobID := strings.TrimSpace(r.PathValue("jobID"))
	record, found, err := h.jobStore.GetByID(jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read job"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "job deletion") {
		return
	}
	if h.runtime != nil {
		if err := h.runtime.DeleteJob(record.JobName); err != nil && !isNotFoundError(err) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes job delete failed: " + err.Error()})
			return
		}
	}
	if err := h.jobStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete job"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) GetJobLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	jobID := strings.TrimSpace(r.PathValue("jobID"))
	record, found, err := h.jobStore.GetByID(jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read job"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "job logs access") {
		return
	}
	logReader, ok := h.runtime.(noryxruntime.JobLogReader)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "job logs not supported by runtime"})
		return
	}
	tailLines := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("tailLines")); raw != "" {
		if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 {
			tailLines = n
		}
	}
	logs, err := logReader.GetJobLogs(record.JobName, tailLines)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "job logs fetch failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jobId":     record.ID,
		"jobName":   record.JobName,
		"projectId": record.ProjectID,
		"podName":   logs.PodName,
		"logs":      logs.Logs,
	})
}

func (h Handlers) syncJobsFromRuntime() {
	discovery, ok := h.runtime.(noryxruntime.JobDiscovery)
	if !ok {
		return
	}
	items, err := discovery.ListJobs()
	if err != nil {
		return
	}
	for _, item := range items {
		if strings.TrimSpace(item.JobID) == "" || strings.TrimSpace(item.ProjectID) == "" {
			continue
		}
		existing, found, err := h.jobStore.GetByID(item.JobID)
		if err != nil {
			continue
		}
		record := job.Job{
			ID:        strings.TrimSpace(item.JobID),
			ProjectID: strings.TrimSpace(item.ProjectID),
			Name:      strings.TrimSpace(item.JobName),
			Image:     strings.TrimSpace(item.Image),
			JobName:   strings.TrimSpace(item.JobName),
			Status:    strings.TrimSpace(item.Status),
			CreatedAt: time.Now().UTC(),
		}
		if found {
			record = existing
			if v := strings.TrimSpace(item.Status); v != "" {
				record.Status = v
			}
			if v := strings.TrimSpace(item.Image); v != "" {
				record.Image = v
			}
		}
		_ = h.jobStore.Upsert(record)
	}
}

func jobBootstrapScript(userArgs []string, attachedRepos []workspaceAttachedRepo, attachedDatasets []workspaceAttachedDataset, minioEndpoint, minioAccessKey, minioSecretKey string, minioUseSSL bool) string {
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
	if len(userArgs) > 0 {
		lines = append(lines, strings.Join(userArgs, " "))
	} else {
		lines = append(lines, "echo 'job finished: no command provided'")
	}
	return strings.Join(lines, "\n")
}

func workspaceDatasetBootstrapLines(attachedDatasets []workspaceAttachedDataset, minioEndpoint, minioAccessKey, minioSecretKey string, minioUseSSL bool) []string {
	pythonSecure := "False"
	if minioUseSSL {
		pythonSecure = "True"
	}
	lines := []string{
		"python3 - <<'PY' || true",
		"import importlib.util, os, site, subprocess, sys",
		"if importlib.util.find_spec('minio') is None:",
		"    subprocess.run([sys.executable, '-m', 'pip', 'install', '--disable-pip-version-check', '--user', 'minio'], check=False)",
		"user_site = site.getusersitepackages()",
		"if user_site and user_site not in sys.path: sys.path.append(user_site)",
		"from minio import Minio",
		fmt.Sprintf("client = Minio(%q, access_key=%q, secret_key=%q, secure=%s)", minioEndpoint, minioAccessKey, minioSecretKey, pythonSecure),
		"datasets = [",
	}
	for _, ds := range attachedDatasets {
		lines = append(lines, fmt.Sprintf("    {'name': %q, 'bucket': %q, 'prefix': %q},", sanitizeWorkspacePathName(ds.Name), strings.TrimSpace(ds.Bucket), strings.Trim(strings.TrimSpace(ds.Prefix), "/")))
	}
	lines = append(lines,
		"]",
		"for ds in datasets:",
		"    target = os.path.join('/datasets', ds['name'])",
		"    os.makedirs(target, exist_ok=True)",
		"    prefix = ds.get('prefix', '').strip('/')",
		"    p = (prefix + '/') if prefix else ''",
		"    for obj in client.list_objects(ds['bucket'], prefix=p, recursive=True):",
		"        o = obj.object_name",
		"        rel = o[len(p):] if p and o.startswith(p) else o",
		"        if not rel or rel.endswith('/'): continue",
		"        dst = os.path.join(target, rel)",
		"        os.makedirs(os.path.dirname(dst), exist_ok=True)",
		"        client.fget_object(ds['bucket'], o, dst)",
		"PY",
	)
	return lines
}
