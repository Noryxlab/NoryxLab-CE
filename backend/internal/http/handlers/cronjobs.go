package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/google/uuid"
)

type createCronJobRequest struct {
	ProjectID    string   `json:"projectId"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Command      []string `json:"command"`
	Args         []string `json:"args"`
	HardwareTier string   `json:"hardwareTier"`
	Schedule     string   `json:"schedule"`
	TimeZone     string   `json:"timeZone"`
}

func (h Handlers) ListCronJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	discovery, ok := h.runtime.(noryxruntime.CronJobDiscovery)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "cronjobs not supported by runtime"})
		return
	}
	items, err := discovery.ListCronJobs()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes cronjob list failed: " + err.Error()})
		return
	}
	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	filtered := make([]noryxruntime.CronJobRuntimeInfo, 0, len(items))
	for _, item := range items {
		if projectFilter != "" && item.ProjectID != projectFilter {
			continue
		}
		if h.hasProjectMembership(userID, item.ProjectID) {
			filtered = append(filtered, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": filtered})
}

func (h Handlers) CreateCronJob(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	var req createCronJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	req.Schedule = strings.TrimSpace(req.Schedule)
	req.TimeZone = strings.TrimSpace(req.TimeZone)
	if req.TimeZone == "" {
		req.TimeZone = "Europe/Paris"
	}
	if req.ProjectID == "" || req.Image == "" || req.Schedule == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId, image and schedule are required"})
		return
	}
	if len(strings.Fields(req.Schedule)) != 5 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule must use the standard five-field cron format"})
		return
	}
	if !h.requireProjectRole(w, req.ProjectID, userID, access.Role.CanLaunchPod, "cronjob creation") {
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
	if h.runtime == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kubernetes runtime is disabled"})
		return
	}
	tier, tierFound := h.resolveHardwareTier(req.HardwareTier)
	if !tierFound {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown hardwareTier"})
		return
	}

	cronJobID := uuid.NewString()
	cronJobName := "cron-" + shortID()
	displayName := req.Name
	if displayName == "" {
		displayName = cronJobName
	}
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
	command := req.Command
	args := req.Args
	if len(command) == 0 {
		command = []string{"/bin/sh", "-lc"}
		args = []string{jobBootstrapScript(req.Args, attachedRepos)}
	}
	userSecretName := cronJobName + "-user-secrets"
	if len(userSecretData) > 0 {
		if err := h.runtime.CreateSecret(noryxruntime.SecretSpec{
			Name: userSecretName,
			Data: userSecretData,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-workload-user-secrets",
				"noryx.io/cronjob-id":    cronJobID,
			},
		}); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes cronjob user secret create failed: " + err.Error()})
			return
		}
	}
	volumes := []noryxruntime.PersistentVolumeClaimMount{
		{ClaimName: "project-" + sanitizeK8sName(req.ProjectID), MountPath: workspaceProjectMountPath},
	}
	datasetVolumes, err := h.ensureDatasetVolumeMounts(attachedDatasets)
	if err != nil {
		_ = h.runtime.DeleteSecret(userSecretName)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to prepare direct S3 dataset mounts: " + err.Error()})
		return
	}
	volumes = append(volumes, datasetVolumes...)
	err = h.runtime.CreateCronJob(noryxruntime.CronJobSpec{
		CronJobName: cronJobName,
		DisplayName: displayName,
		Schedule:    req.Schedule,
		TimeZone:    req.TimeZone,
		JobSpec: noryxruntime.JobSpec{
			JobName:                 cronJobName,
			Image:                   req.Image,
			Command:                 command,
			Args:                    args,
			Env:                     append(datasourceEnv, secretEnvRefs(userSecretName, userSecretData)...),
			CPURequest:              tier.CPURequest,
			CPULimit:                tier.CPULimit,
			MemRequest:              tier.MemoryRequest,
			MemLimit:                tier.MemoryLimit,
			EphemeralStorageRequest: tier.EphemeralRequest,
			EphemeralStorageLimit:   tier.EphemeralStorageLimit,
			PullSecret:              h.registryPullSecret,
			Volumes:                 volumes,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-cronjob",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/cronjob-id":    cronJobID,
				"noryx.io/hardware-tier": tier.ID,
			},
		},
	})
	if err != nil {
		_ = h.runtime.DeleteSecret(userSecretName)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes cronjob create failed: " + err.Error()})
		return
	}
	h.emitAudit(r, userID, "cronjob.create", "cronjob", cronJobID, req.ProjectID, "success", "", map[string]any{
		"name": displayName, "cronJobName": cronJobName, "schedule": req.Schedule, "timeZone": req.TimeZone, "image": req.Image,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": cronJobID, "projectId": req.ProjectID, "name": displayName, "cronJobName": cronJobName,
		"schedule": req.Schedule, "timeZone": req.TimeZone, "image": req.Image,
	})
}

func (h Handlers) DeleteCronJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	cronJobID := strings.TrimSpace(r.PathValue("cronJobID"))
	discovery, ok := h.runtime.(noryxruntime.CronJobDiscovery)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "cronjobs not supported by runtime"})
		return
	}
	items, err := discovery.ListCronJobs()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes cronjob list failed: " + err.Error()})
		return
	}
	var target noryxruntime.CronJobRuntimeInfo
	found := false
	for _, item := range items {
		if item.CronJobID == cronJobID {
			target, found = item, true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cronjob not found"})
		return
	}
	if !h.requireProjectRole(w, target.ProjectID, userID, access.Role.CanLaunchPod, "cronjob deletion") {
		return
	}
	if err := h.runtime.DeleteCronJob(target.CronJobName); err != nil && !isNotFoundError(err) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes cronjob delete failed: " + err.Error()})
		return
	}
	if err := h.runtime.DeleteSecret(target.CronJobName + "-user-secrets"); err != nil && !isNotFoundError(err) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes cronjob user secret delete failed: " + err.Error()})
		return
	}
	h.emitAudit(r, userID, "cronjob.delete", "cronjob", target.CronJobID, target.ProjectID, "success", "", map[string]any{
		"name": target.Name, "cronJobName": target.CronJobName,
	})
	w.WriteHeader(http.StatusNoContent)
}
