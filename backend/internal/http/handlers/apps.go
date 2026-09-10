package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"golang.org/x/text/unicode/norm"
)

type createAppRequest struct {
	ProjectID            string   `json:"projectId"`
	Name                 string   `json:"name"`
	Slug                 string   `json:"slug"`
	Image                string   `json:"image"`
	Command              []string `json:"command"`
	Args                 []string `json:"args"`
	Port                 int      `json:"port"`
	HardwareTier         string   `json:"hardwareTier"`
	AccessMode           string   `json:"accessMode"`
	AllowedUsers         []string `json:"allowedUsers"`
	AllowedOrganizations []string `json:"allowedOrganizations"`
}

var appSlugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,30}[a-z0-9])?$`)

func (h Handlers) ListApps(w http.ResponseWriter, r *http.Request) {
	h.listAppsByKind(w, r, "app")
}

func (h Handlers) ListDashboards(w http.ResponseWriter, r *http.Request) {
	h.listAppsByKind(w, r, "dashboard")
}

func (h Handlers) ListAPIs(w http.ResponseWriter, r *http.Request) {
	h.listAppsByKind(w, r, "api")
}

func (h Handlers) listAppsByKind(w http.ResponseWriter, r *http.Request, kind string) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.appStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}
	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	filtered := make([]app.App, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Kind) == "" {
			item.Kind = "app"
		}
		if item.Kind != kind {
			continue
		}
		if projectFilter != "" && item.ProjectID != projectFilter {
			continue
		}
		if !h.hasProjectMembership(userID, item.ProjectID) {
			continue
		}
		item = h.enrichAppRuntimeStatus(item)
		filtered = append(filtered, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": filtered})
}

func (h Handlers) enrichAppRuntimeStatus(item app.App) app.App {
	if podOperator, ok := h.runtime.(noryxruntime.PodOperator); ok && strings.TrimSpace(item.PodName) != "" {
		status, err := podOperator.GetPodStatus(item.PodName)
		if err == nil {
			item.RestartCount = status.RestartCount
			if !status.StartedAt.IsZero() {
				startedAt := status.StartedAt
				item.StartedAt = &startedAt
			}
			item.HealthMessage = strings.TrimSpace(status.Reason + " " + status.Message)
			switch status.Phase {
			case "failed":
				item.Status = "failed"
			case "succeeded":
				item.Status = "stopped"
			case "pending":
				item.Status = "launching"
			case "running":
				item.Status = "unhealthy"
			case "unknown":
				// Kubernetes says it cannot reach the pod's node, so it does not
				// know whether the application is alive. Left unhandled, this
				// case fell through and the stored value survived: applications
				// whose node had been gone for forty-four days were still
				// reported as running, and a call to them answered 502 from a
				// screen that said everything was fine.
				item.Status = "unchecked"
			}
		} else if isNotFoundError(err) {
			item.Status = "stopped"
		}
	}
	if readiness, ok := h.runtime.(noryxruntime.WorkspaceReadiness); ok && strings.TrimSpace(item.ServiceName) != "" && item.Status != "stopped" && item.Status != "failed" {
		ready, err := readiness.IsServiceReady(item.ServiceName)
		if err == nil {
			if ready {
				item.Status = "running"
			} else {
				item.Status = "launching"
			}
		}
	}
	return item
}

func (h Handlers) GetAppLogs(w http.ResponseWriter, r *http.Request) {
	record, userID, ok := h.requireAppOperation(w, r, "app logs")
	if !ok {
		return
	}
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "app logs are not supported by runtime"})
		return
	}
	tailLines := 300
	if raw := strings.TrimSpace(r.URL.Query().Get("tailLines")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &tailLines); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tailLines must be an integer"})
			return
		}
	}
	logs, err := operator.GetPodLogs(record.PodName, tailLines)
	if err != nil {
		// An app whose pod is gone still has something to say, and one that has
		// not started yet is not an error - the same distinction build logs
		// have always made, and the reason a rebuild can be watched while an
		// app could only be refreshed by hand.
		if h.appIsStarting(record) {
			writeJSON(w, http.StatusOK, map[string]any{
				"appId": record.ID, "podName": record.PodName,
				"logs": "", "pending": true,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"appId": record.ID, "podName": record.PodName, "logs": "",
			"unavailable": "the application's pod is gone, so its output is no longer available",
		})
		return
	}
	h.emitAudit(r, userID, "app.logs.read", record.Kind, record.ID, record.ProjectID, "success", "", map[string]any{"name": record.Name})
	// pending while the pod runs but has not been declared ready: the launch
	// command has not printed its last line yet, so a watcher keeps polling.
	writeJSON(w, http.StatusOK, map[string]any{
		"appId": record.ID, "podName": record.PodName, "logs": logs,
		"pending": h.appIsStarting(record) && !h.appIsServing(record),
	})
}

// RestartApp relaunches an application from what the platform knows about it,
// rather than from the pod that happens to be running.
//
// It used to read the live pod and put it back. Two consequences, both met in
// production on the same afternoon:
//
//   - an application that had been *stopped* could never start again. Stopping
//     deletes the pod, so the read failed and the restart answered 502 forever.
//     The only way back was to delete the application and recreate it, losing
//     its identity and its history.
//   - a restart reused the dataset volumes exactly as they were. The S3
//     endpoint is resolved to an address when the volume is written, because
//     the mounter runs on the host and cannot resolve cluster DNS; when the
//     MinIO service was recreated with a new address, every restart rebuilt a
//     pod pointing at an address nobody answered. The button that exists to
//     repair an application could not repair that one.
//
// Relaunching from the record fixes both: the volumes are re-resolved the way
// creation resolves them, and nothing depends on a pod still being there.
func (h Handlers) RestartApp(w http.ResponseWriter, r *http.Request) {
	h.restartAppByKind(w, r, "app")
}

// RestartDashboard is the same operation for a dashboard.
//
// Dashboards had no restart at all - no stop, no logs, no revisions either -
// so a dashboard whose pod died could only be deleted and recreated, under a
// new identity. They are the same workload as an application and now share its
// lifecycle.
func (h Handlers) RestartDashboard(w http.ResponseWriter, r *http.Request) {
	h.restartAppByKind(w, r, "dashboard")
}

// RestartAPI is the same operation for a deployed endpoint.
func (h Handlers) RestartAPI(w http.ResponseWriter, r *http.Request) {
	h.restartAppByKind(w, r, "api")
}

func (h Handlers) restartAppByKind(w http.ResponseWriter, r *http.Request, kind string) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	record, userID, ok := h.requireAppOperation(w, r, kind+" restart")
	if !ok {
		return
	}
	if h.runtime == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "restart is not supported by runtime"})
		return
	}
	if err := h.relaunchApp(record, identity, userID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to restart: " + err.Error()})
		return
	}
	record.Status = "launching"
	_ = h.appStore.Upsert(record)
	h.emitAudit(r, userID, record.Kind+".restart", record.Kind, record.ID, record.ProjectID, "success", "", map[string]any{"name": record.Name})
	writeJSON(w, http.StatusAccepted, record)
}

// waitForPodToDisappear blocks until the pod's name is free again.
//
// Bounded: a pod that will not go is a condition to report, not one to wait on
// forever while a caller holds a request open.
func (h Handlers) waitForPodToDisappear(podName string) error {
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		return nil
	}
	deadline := time.Now().Add(podDeletionTimeout)
	for {
		if _, err := operator.GetPodStatus(podName); err != nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the previous instance is still terminating after %s", podDeletionTimeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// podDeletionTimeout is generous: a container with a long stop hook legitimately
// takes seconds, and the alternative to waiting is a restart that fails.
const podDeletionTimeout = 45 * time.Second

// relaunchApp rebuilds the workload from the stored record.
//
// The bootstrap script itself is reused rather than regenerated: the record
// keeps the generated script, not the command the user typed, so regenerating
// it is not possible. What must be rebuilt is everything around it - above all
// the dataset volumes, whose endpoint goes stale.
func (h Handlers) relaunchApp(record app.App, identity auth.Identity, userID string) error {
	tier, _ := h.resolveHardwareTier(record.HardwareTier)

	_, attachedDatasets, err := h.resolveProjectWorkspaceResources(record.ProjectID, identity, true)
	if err != nil {
		return fmt.Errorf("resolve project resources: %w", err)
	}
	datasourceEnv, err := h.resolveProjectDatasourceEnv(record.ProjectID, userID)
	if err != nil {
		return fmt.Errorf("resolve project datasources: %w", err)
	}
	userSecretData, err := h.workloadEnvData(record.ProjectID, userID)
	if err != nil {
		return fmt.Errorf("resolve user secrets: %w", err)
	}

	userSecretName := record.PodName + "-user-secrets"
	if len(userSecretData) > 0 {
		if err := h.runtime.CreateSecret(noryxruntime.SecretSpec{
			Name: userSecretName,
			Data: userSecretData,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-workload-user-secrets",
				"noryx.io/app-id":        record.ID,
			},
		}); err != nil {
			return fmt.Errorf("user secret: %w", err)
		}
	}
	volumes, err := h.ensureProjectVolume(record.ProjectID)
	if err != nil {
		return fmt.Errorf("project volume: %w", err)
	}
	// The reason this function exists: the endpoint is re-resolved here.
	datasetVolumes, err := h.ensureDatasetVolumeMounts(attachedDatasets)
	if err != nil {
		return fmt.Errorf("dataset mounts: %w", err)
	}
	volumes = append(volumes, datasetVolumes...)

	// Deleting first, and tolerating its absence: a stopped application has no
	// pod, and that is the case this whole function exists to serve.
	_ = h.runtime.DeletePod(record.PodName)
	// And waiting for it to be gone. Deletion is asynchronous: the pod keeps
	// its name while it terminates, so recreating it immediately came back as
	// "already exists" and the restart failed with a 409 wrapped in a 502 -
	// which reads like a broken cluster and is nothing but impatience.
	if err := h.waitForPodToDisappear(record.PodName); err != nil {
		return err
	}

	if err := h.runtime.CreatePod(noryxruntime.PodSpec{
		PodName:                 record.PodName,
		Image:                   record.Image,
		Command:                 record.Command,
		Args:                    record.Args,
		Env:                     append(datasourceEnv, secretEnvRefs(userSecretName, userSecretData)...),
		Ports:                   []int{record.Port},
		ReadinessPort:           record.Port,
		CPURequest:              tier.CPURequest,
		CPULimit:                tier.CPULimit,
		MemRequest:              tier.MemoryRequest,
		MemLimit:                tier.MemoryLimit,
		EphemeralStorageRequest: tier.EphemeralStorageRequest,
		EphemeralStorageLimit:   tier.EphemeralStorageLimit,
		PullSecret:              h.registryPullSecret,
		Volumes:                 volumes,
		Labels: map[string]string{
			"app.kubernetes.io/name": "noryx-app",
			"noryx.io/project-id":    record.ProjectID,
			"noryx.io/app-id":        record.ID,
			"noryx.io/app-kind":      record.Kind,
			"noryx.io/app-slug":      record.Slug,
			"noryx.io/app-pod":       record.PodName,
			"noryx.io/hardware-tier": tier.ID,
		},
	}); err != nil {
		return fmt.Errorf("pod: %w", err)
	}

	// The service outlives a stop, so it is normally already there - and an
	// application restarted after its service was removed must come back whole.
	// Both cases are the same instruction: make sure it exists.
	//
	// Its presence is therefore not an error. Treating the conflict as a
	// failure meant a restart recreated the pod, failed on the service that was
	// fine, and reported a broken cluster while leaving the application
	// half-relaunched.
	err = h.runtime.CreateService(noryxruntime.ServiceSpec{
		Name:     record.ServiceName,
		Selector: map[string]string{"noryx.io/app-pod": record.PodName},
		Port:     record.Port,
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("service: %w", err)
	}
	return nil
}

// isAlreadyExists reports the one Kubernetes refusal that means "nothing to do".
func isAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status=409")
}

func (h Handlers) StopApp(w http.ResponseWriter, r *http.Request) {
	record, userID, ok := h.requireAppOperation(w, r, "app stop")
	if !ok {
		return
	}
	if h.runtime != nil {
		if err := h.runtime.DeletePod(record.PodName); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to stop app: " + err.Error()})
			return
		}
	}
	record.Status = "stopped"
	if err := h.appStore.Upsert(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist stopped app"})
		return
	}
	h.emitAudit(r, userID, "app.stop", record.Kind, record.ID, record.ProjectID, "success", "", map[string]any{"name": record.Name})
	writeJSON(w, http.StatusOK, record)
}

// kindFromRoute names the workload kind the matched route addresses, taken
// from which id segment the route declared.
func kindFromRoute(r *http.Request) string {
	switch {
	case strings.TrimSpace(r.PathValue("dashboardID")) != "":
		return "dashboard"
	case strings.TrimSpace(r.PathValue("apiID")) != "":
		return "api"
	case strings.TrimSpace(r.PathValue("appID")) != "":
		return "app"
	}
	return ""
}

// requireAppOperation resolves the workload an operation addresses.
//
// It reads whichever id the route carries, because the three kinds are the same
// workload under three names and each has its own path segment. It used to read
// only "appID", which is why a dashboard or an API route could never have found
// its own record even once its handler existed.
func (h Handlers) requireAppOperation(w http.ResponseWriter, r *http.Request, operation string) (app.App, string, bool) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return app.App{}, "", false
	}
	id := ""
	for _, name := range []string{"appID", "dashboardID", "apiID"} {
		if value := strings.TrimSpace(r.PathValue(name)); value != "" {
			id = value
			break
		}
	}
	record, found, err := h.appStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app"})
		return app.App{}, "", false
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return app.App{}, "", false
	}
	// The route says which kind it addresses, and a record of another kind is
	// not found rather than refused: /api/v1/dashboards/<id of an app> must not
	// operate on that application.
	if expectedKind := kindFromRoute(r); expectedKind != "" {
		actual := strings.TrimSpace(record.Kind)
		if actual == "" {
			actual = "app"
		}
		if actual != expectedKind {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return app.App{}, "", false
		}
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, actionLaunch, operation) {
		return app.App{}, "", false
	}
	return record, userID, true
}

func (h Handlers) CreateApp(w http.ResponseWriter, r *http.Request) {
	h.createAppByKind(w, r, "app")
}

func (h Handlers) CreateDashboard(w http.ResponseWriter, r *http.Request) {
	h.createAppByKind(w, r, "dashboard")
}

func (h Handlers) CreateAPI(w http.ResponseWriter, r *http.Request) {
	h.createAppByKind(w, r, "api")
}

func (h Handlers) createAppByKind(w http.ResponseWriter, r *http.Request, kind string) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = normalizeAppSlug(req.Slug)
	req.Image = strings.TrimSpace(req.Image)
	req.AccessMode = strings.ToLower(strings.TrimSpace(req.AccessMode))
	if req.AccessMode == "" {
		req.AccessMode = "private"
	}
	// A dashboard's access is decided below, not by the caller, so its value is
	// not judged here.
	//
	// The platform stored every dashboard as "project" and then refused that
	// same value on the way in, which made a stored dashboard impossible to
	// recreate from what the platform itself had recorded about it.
	if kind == "dashboard" {
		req.AccessMode = "project"
	} else if req.AccessMode != "public" && req.AccessMode != "organization" && req.AccessMode != "users" && req.AccessMode != "private" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "accessMode must be public, organization, users, or private"})
		return
	}
	if req.AccessMode == "organization" && len(req.AllowedOrganizations) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "allowedOrganizations is required for organization access"})
		return
	}
	for index, organizationID := range req.AllowedOrganizations {
		organization, found := h.resolveOrganization(organizationID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no organization named " + strings.TrimSpace(organizationID)})
			return
		}
		req.AllowedOrganizations[index] = organization.ID
	}
	if req.AccessMode == "users" && len(req.AllowedUsers) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "allowedUsers is required for users access"})
		return
	}
	tier, tierFound := h.resolveHardwareTier(req.HardwareTier)
	if !tierFound {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown hardwareTier"})
		return
	}
	if req.ProjectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	if req.Name == "" {
		if kind == "dashboard" {
			req.Name = "dashboard-" + shortID()
		} else {
			req.Name = "app-" + shortID()
		}
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
	if !h.requireProjectRole(w, req.ProjectID, userID, actionLaunch, "app launch") {
		return
	}
	if existing, found, err := h.appStore.GetBySlug(req.Slug); err == nil && found {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "slug already used by app " + existing.ID})
		return
	}

	prefix := "app"
	if kind == "dashboard" {
		prefix = "dash"
	}
	podName := prefix + "-" + shortID()
	serviceName := podName
	accessURL := "/apps/" + req.Slug + "/"
	switch kind {
	case "dashboard":
		accessURL = "/dashboards/" + req.Slug + "/"
	case "api":
		// No trailing slash: this address is pasted into another system's
		// configuration, and a caller appends its own path to it.
		accessURL = "/apis/" + req.Slug
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
	userSecretData, err := h.workloadEnvData(req.ProjectID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve user secrets"})
		return
	}

	command := []string{"/bin/sh", "-lc"}
	bootstrapScript := appBootstrapScript(req.Port, append(append([]string{}, req.Command...), req.Args...), attachedRepos)
	args := []string{bootstrapScript}

	record := app.NewWithKind(kind, req.ProjectID, req.Name, req.Slug, req.Image, command, args, req.Port, podName, serviceName, accessURL)
	record.OwnerUserID = userID
	record.AccessMode = req.AccessMode
	record.AllowedUsers = normalizeAppSubjects(req.AllowedUsers)
	record.AllowedOrganizations = normalizeAppSubjects(req.AllowedOrganizations)
	// Kept, so the app's share of the cluster can be counted while it runs.
	record.HardwareTier = tier.ID
	if kind == "dashboard" {
		record.AccessMode = "project"
	}

	if h.runtime != nil {
		userSecretName := podName + "-user-secrets"
		if len(userSecretData) > 0 {
			if err := h.runtime.CreateSecret(noryxruntime.SecretSpec{
				Name: userSecretName,
				Data: userSecretData,
				Labels: map[string]string{
					"app.kubernetes.io/name": "noryx-workload-user-secrets",
					"noryx.io/app-id":        record.ID,
				},
			}); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes app user secret create failed: " + err.Error()})
				return
			}
		}
		volumes, err := h.ensureProjectVolume(req.ProjectID)
		if err != nil {
			_ = h.runtime.DeleteSecret(userSecretName)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to prepare project volume: " + err.Error()})
			return
		}
		datasetVolumes, err := h.ensureDatasetVolumeMounts(attachedDatasets)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to prepare direct S3 dataset mounts: " + err.Error()})
			return
		}
		volumes = append(volumes, datasetVolumes...)
		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName: podName,
			Image:   record.Image,
			Command: command,
			Args:    args,
			Env:     append(datasourceEnv, secretEnvRefs(userSecretName, userSecretData)...),
			Ports:   []int{record.Port},
			// The service only routes here once something answers on the port.
			//
			// Without it, an application is an endpoint of its service from the
			// instant the container starts - which is minutes before anything
			// listens, because the dependencies are installed first. Every call
			// in that window reached a closed port and came back as a connection
			// refused, and the platform had no way to tell "starting" from
			// "broken" because Kubernetes reported the pod as running either way.
			ReadinessPort:           record.Port,
			CPURequest:              tier.CPURequest,
			CPULimit:                tier.CPULimit,
			MemRequest:              tier.MemoryRequest,
			MemLimit:                tier.MemoryLimit,
			EphemeralStorageRequest: tier.EphemeralStorageRequest,
			EphemeralStorageLimit:   tier.EphemeralStorageLimit,
			PullSecret:              h.registryPullSecret,
			Volumes:                 volumes,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-app",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/app-id":        record.ID,
				"noryx.io/app-kind":      kind,
				"noryx.io/app-slug":      req.Slug,
				"noryx.io/app-pod":       podName,
				"noryx.io/hardware-tier": tier.ID,
			},
		})
		if err != nil {
			_ = h.runtime.DeleteSecret(userSecretName)
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
			_ = h.runtime.DeleteSecret(userSecretName)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes app service creation failed: " + err.Error()})
			return
		}
	}
	if err := h.appStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save app"})
		return
	}
	action := "app.launch"
	if record.Kind == "dashboard" {
		action = "dashboard.launch"
	}
	h.emitAudit(r, userID, action, record.Kind, record.ID, record.ProjectID, "success", "", map[string]any{
		"name":         record.Name,
		"slug":         record.Slug,
		"image":        record.Image,
		"port":         record.Port,
		"hardwareTier": tier.ID,
		"accessMode":   record.AccessMode,
	})
	writeJSON(w, http.StatusCreated, record)
}

func normalizeAppSubjects(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (h Handlers) DeleteApp(w http.ResponseWriter, r *http.Request) {
	h.deleteAppByKind(w, r, "app")
}

func (h Handlers) DeleteDashboard(w http.ResponseWriter, r *http.Request) {
	h.deleteAppByKind(w, r, "dashboard")
}

func (h Handlers) DeleteAPI(w http.ResponseWriter, r *http.Request) {
	h.deleteAppByKind(w, r, "api")
}

func (h Handlers) deleteAppByKind(w http.ResponseWriter, r *http.Request, kind string) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	appID := strings.TrimSpace(r.PathValue("appID"))
	if appID == "" {
		appID = strings.TrimSpace(r.PathValue("dashboardID"))
	}
	record, found, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if strings.TrimSpace(record.Kind) == "" {
		record.Kind = "app"
	}
	if record.Kind != kind {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resource not found"})
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, actionLaunch, "app deletion") {
		return
	}
	if h.runtime != nil {
		_ = h.runtime.DeleteService(record.ServiceName)
		_ = h.runtime.DeletePod(record.PodName)
		_ = h.runtime.DeleteSecret(record.PodName + "-user-secrets")
	}
	if err := h.appStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete app"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
	action := "app.delete"
	if record.Kind == "dashboard" {
		action = "dashboard.delete"
	}
	h.emitAudit(r, userID, action, record.Kind, record.ID, record.ProjectID, "success", "", map[string]any{
		"name":       record.Name,
		"slug":       record.Slug,
		"accessMode": record.AccessMode,
	})
}

func normalizeAppSlug(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(norm.NFD.String(raw)))
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
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

// appBootstrapScript writes the script the container runs.
//
// launchArgv is the launch command as *words*, which is what both callers
// actually hold: the form splits what the user typed on whitespace, and the
// API takes command and args as arrays. Joining them into one line and letting
// the shell split it again lost every word boundary that mattered - an
// application declared as {"command":["/bin/sh","-lc"],"args":["FOO=1 run.sh"]}
// became `exec /bin/sh -lc FOO=1 run.sh`, where sh takes only the first word as
// its command string. It ran the assignment, exited 0, and the platform showed
// a container that had "succeeded".
// boolShell renders a constant test, so the branch is decided here rather than
// by a string comparison the shell has to redo at every launch.
func boolShell(yes bool) string {
	if yes {
		return "true"
	}
	return "false"
}

func appBootstrapScript(port int, launchArgv []string, attachedRepos []workspaceAttachedRepo) string {
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
		lines = append(lines, repositoryBootstrapLines(repo, repoDir)...)
	}
	launch := make([]string, 0, len(launchArgv))
	for _, word := range launchArgv {
		if strings.TrimSpace(word) != "" {
			launch = append(launch, shellQuote(word))
		}
	}
	defaultHTTP := fmt.Sprintf("python3 -m http.server %d --bind 0.0.0.0 --directory /mnt", port)
	lines = append(lines,
		// Where the requirements were just installed.
		//
		// Without this the platform installed streamlit and then ran a command
		// that could not find it: pip puts console scripts in the project venv
		// and in ~/.local/bin, neither of which is on PATH by default. The
		// workspace bootstrap has always done this; the app bootstrap did not,
		// so the same requirements.txt worked in a workspace and failed in an
		// app with "streamlit: not found" - after installing streamlit.
		fmt.Sprintf("if [ -x %s/bin/python ]; then export PATH=%s/bin:$PATH; fi", workspaceProjectVenvPath, workspaceProjectVenvPath),
		"export PATH=$HOME/.local/bin:$PATH",
		fmt.Sprintf("export PORT=%d NORYX_APP_PORT=%d", port, port),
		// The project directory is the application's working directory.
		//
		// The launch command is written the way anybody writes one - `streamlit
		// run weather_app.py` - and was run from wherever the image happens to
		// start, so a file sitting in the project failed with "File does not
		// exist" while being right there. The static-server fallback already
		// served /mnt and /mnt/app.sh was already the other entrypoint; the
		// user's own command was the one case that did not get the same footing.
		fmt.Sprintf("cd %s || true", workspaceProjectMountPath),
		"if [ "+boolShell(len(launch) > 0)+" ]; then",
		"  echo '[bootstrap] using UI command entrypoint'",
		// exec: the command becomes the container's process, so a stop signal
		// reaches the server instead of the shell that started it. Each word is
		// quoted, so a word carrying spaces stays one word.
		"  exec "+strings.Join(launch, " "),
		"elif [ -f /mnt/app.sh ]; then",
		"  echo '[bootstrap] using /mnt/app.sh entrypoint'",
		"  chmod +x /mnt/app.sh || true",
		"  exec /mnt/app.sh",
		"else",
		"  echo '[bootstrap] no /mnt/app.sh found, using default static server'",
		"  "+defaultHTTP,
		"fi",
	)
	return strings.Join(lines, "\n")
}
