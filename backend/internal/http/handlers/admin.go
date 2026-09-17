package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type adminExecution struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	ProjectID   string    `json:"projectId"`
	ProjectName string    `json:"projectName"`
	RuntimeName string    `json:"runtimeName"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (h Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}

	users, err := h.keycloak.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to fetch users from keycloak: " + err.Error()})
		return
	}

	// Merged here rather than asked of Keycloak: Keycloak knows who may sign
	// in, the platform knows who did. An account that exists and has never been
	// used looks identical to a working one on this screen otherwise, and that
	// is exactly the pair an access review has to tell apart.
	//
	// A failure to read it degrades the column, not the page: the list of
	// accounts is the thing being asked for.
	type userRow struct {
		keycloak.User
		// Organization, because an administration screen for an installation
		// shared between several parties is read by party before it is read by
		// person.
		Organization string `json:"organization,omitempty"`
		// Administrator marks the accounts that can act on everybody else's
		// work. An administration screen that does not distinguish them is one
		// where the most consequential fact about a row is the one it omits.
		Administrator bool       `json:"administrator,omitempty"`
		LastSeenAt    *time.Time `json:"lastSeenAt,omitempty"`
		// No sign-in count here, deliberately.
		//
		// It was reported alongside the date and it reads as surveillance on a
		// row about one named person: "8 sign-ins" next to somebody's name
		// answers a question nobody asked of this screen. The question this
		// screen asks is whether the account is in use, which the date alone
		// answers. How much anybody used the platform belongs to the activity
		// report, where it is aggregated and where it is the point.
	}
	rows := make([]userRow, 0, len(users))
	membership := h.actorOrganizations()
	administrators := h.globalAdministrators()
	seen := map[string]store.LastSeen{}
	if h.auditStore != nil {
		if found, err := h.auditStore.LastSeen(); err != nil {
			log.Printf("users: last-seen unavailable, listing without it: %v", err)
		} else {
			seen = found
		}
	}
	for _, user := range users {
		row := userRow{User: user}
		row.Administrator = administrators[strings.ToLower(user.Username)] ||
			administrators[strings.ToLower(user.Email)]
		if organization := membership[strings.ToLower(user.Username)]; organization != "" {
			row.Organization = organization
		} else {
			row.Organization = membership[strings.ToLower(user.Email)]
		}
		// The audit records the actor as the username, which is what
		// Identity.UserID resolves to. Email is the fallback for an account
		// that has no username, and is checked second for the same reason.
		entry, ok := seen[user.Username]
		if !ok {
			entry, ok = seen[user.Email]
		}
		if ok && !entry.At.IsZero() {
			at := entry.At
			row.LastSeenAt = &at
		}
		rows = append(rows, row)
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h Handlers) GetModulesStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdminModule(w, r, "modules")
	if !ok {
		return
	}

	inspector, ok := h.runtime.(noryxruntime.Inspector)
	if !ok || inspector == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "runtime inspector is not available"})
		return
	}

	deployments, err := inspector.ListDeployments()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list deployments: " + err.Error()})
		return
	}
	services, err := inspector.ListServices()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list services: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deployments": deployments,
		"services":    services,
	})
}

func (h Handlers) GetAdminOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "overview"); !ok {
		return
	}
	projects, projectErr := h.projectStore.List()
	datasets, datasetErr := h.datasetStore.ListAll()
	workspaces, workspaceErr := h.workspaceStore.List()
	apps, appErr := h.appStore.List()
	jobs, jobErr := h.jobStore.List()
	builds, buildErr := h.buildStore.List()
	if projectErr != nil || datasetErr != nil || workspaceErr != nil || appErr != nil || jobErr != nil || buildErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build admin overview"})
		return
	}
	datasets = h.filterDatasetsForEdition(datasets)
	userCount := 0
	if h.keycloak != nil {
		if users, err := h.keycloak.ListUsers(); err == nil {
			userCount = len(users)
		}
	}
	active := 0
	for _, status := range appendExecutionStatuses(workspaces, apps, jobs, builds) {
		if status == "running" || status == "launching" || status == "submitted" {
			active++
		}
	}
	metrics := noryxruntime.WorkloadMetrics{}
	if inspector, ok := h.runtime.(noryxruntime.WorkloadMetricsInspector); ok {
		metrics, _ = inspector.GetWorkloadMetrics()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"counts": map[string]int{
			"users":      userCount,
			"projects":   len(projects),
			"datasets":   len(datasets),
			"workspaces": len(workspaces),
			"apps":       len(apps),
			"jobs":       len(jobs),
			"builds":     len(builds),
			"active":     active,
		},
		"workloadMetrics": metrics,
	})
}

func appendExecutionStatuses(workspaces []workspace.Workspace, apps []app.App, jobs []job.Job, builds []build.Build) []string {
	out := make([]string, 0, len(workspaces)+len(apps)+len(jobs)+len(builds))
	for _, item := range workspaces {
		out = append(out, strings.ToLower(strings.TrimSpace(item.Status)))
	}
	for _, item := range apps {
		out = append(out, strings.ToLower(strings.TrimSpace(item.Status)))
	}
	for _, item := range jobs {
		out = append(out, strings.ToLower(strings.TrimSpace(item.Status)))
	}
	for _, item := range builds {
		out = append(out, strings.ToLower(strings.TrimSpace(item.Status)))
	}
	return out
}

func (h Handlers) GetAdminInventory(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "inventory"); !ok {
		return
	}
	projects, err := h.projectStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}
	datasets, err := h.datasetStore.ListAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasets"})
		return
	}
	datasets = h.filterDatasetsForEdition(datasets)
	users := any([]any{})
	if h.keycloak != nil {
		if keycloakUsers, err := h.keycloak.ListUsers(); err == nil {
			users = keycloakUsers
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects, "datasets": datasets, "users": users})
}

func (h Handlers) ListAdminExecutions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "executions"); !ok {
		return
	}

	h.syncBuildsFromRuntime()
	h.syncJobsFromRuntime()

	projectNames := map[string]string{}
	projects, err := h.projectStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}
	for _, item := range projects {
		projectNames[item.ID] = item.Name
	}

	items := []adminExecution{}
	readiness, hasReadiness := h.runtime.(noryxruntime.WorkspaceReadiness)

	workspaces, err := h.workspaceStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list workspaces"})
		return
	}
	for _, item := range workspaces {
		status := item.Status
		if hasReadiness && item.ServiceName != "" {
			if ready, err := readiness.IsServiceReady(item.ServiceName); err == nil {
				if ready {
					status = "running"
				} else {
					status = "launching"
				}
			}
		}
		items = append(items, adminExecution{item.ID, "workspace", item.Name, item.ProjectID, projectNames[item.ProjectID], item.PodName, status, item.CreatedAt})
	}

	apps, err := h.appStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}
	for _, item := range apps {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			kind = "app"
		}
		status := item.Status
		if hasReadiness && item.ServiceName != "" {
			if ready, err := readiness.IsServiceReady(item.ServiceName); err == nil {
				if ready {
					status = "running"
				} else {
					status = "launching"
				}
			}
		}
		items = append(items, adminExecution{item.ID, kind, item.Name, item.ProjectID, projectNames[item.ProjectID], item.PodName, status, item.CreatedAt})
	}

	jobs, err := h.jobStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list jobs"})
		return
	}
	for _, item := range jobs {
		items = append(items, adminExecution{item.ID, "job", item.Name, item.ProjectID, projectNames[item.ProjectID], item.JobName, item.Status, item.CreatedAt})
	}

	builds, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}
	for _, item := range builds {
		items = append(items, adminExecution{item.ID, "build", item.DestinationImage, item.ProjectID, projectNames[item.ProjectID], item.JobName, item.Status, item.CreatedAt})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) StopAdminExecution(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "executions")
	if !ok {
		return
	}
	kind := strings.ToLower(strings.TrimSpace(r.PathValue("kind")))
	id := strings.TrimSpace(r.PathValue("executionID"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "executionID is required"})
		return
	}

	projectID, name, err := h.stopAdminExecution(kind, id)
	if err != nil {
		status := http.StatusBadGateway
		if isNotFoundError(err) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "admin.execution.stop", kind, id, projectID, "success", "", map[string]any{"name": name})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) stopAdminExecution(kind, id string) (string, string, error) {
	switch kind {
	case "workspace":
		item, found, err := h.workspaceStore.GetByID(id)
		if err != nil {
			return "", "", err
		}
		if !found {
			return "", "", fmt.Errorf("workspace not found")
		}
		if h.runtime != nil {
			for _, err := range []error{h.runtime.DeleteService(item.ServiceName), h.runtime.DeletePod(item.PodName), h.runtime.DeleteSecret(item.PodName + "-bootstrap"), h.runtime.DeleteSecret(item.PodName + "-user-secrets")} {
				if err != nil && !isNotFoundError(err) {
					return "", "", err
				}
			}
		}
		return item.ProjectID, item.Name, h.workspaceStore.Delete(item.ID)
	case "app", "dashboard":
		item, found, err := h.appStore.GetByID(id)
		if err != nil {
			return "", "", err
		}
		if !found {
			return "", "", fmt.Errorf("%s not found", kind)
		}
		if h.runtime != nil {
			for _, err := range []error{h.runtime.DeleteService(item.ServiceName), h.runtime.DeletePod(item.PodName)} {
				if err != nil && !isNotFoundError(err) {
					return "", "", err
				}
			}
		}
		return item.ProjectID, item.Name, h.appStore.Delete(item.ID)
	case "job":
		item, found, err := h.jobStore.GetByID(id)
		if err != nil {
			return "", "", err
		}
		if !found {
			return "", "", fmt.Errorf("job not found")
		}
		if h.runtime != nil {
			if err := h.runtime.DeleteJob(item.JobName); err != nil && !isNotFoundError(err) {
				return "", "", err
			}
		}
		return item.ProjectID, item.Name, h.jobStore.Delete(item.ID)
	case "build":
		item, found, err := h.buildStore.GetByID(id)
		if err != nil {
			return "", "", err
		}
		if !found {
			return "", "", fmt.Errorf("build not found")
		}
		if h.runtime != nil {
			if err := h.runtime.DeleteJob(item.JobName); err != nil && !isNotFoundError(err) {
				return "", "", err
			}
		}
		return item.ProjectID, item.DestinationImage, h.buildStore.Delete(item.ID)
	default:
		return "", "", fmt.Errorf("execution kind not found")
	}
}

// globalAdministrators is who may act on everybody else's work.
//
// Read from the realm role and from the bootstrap account, which are the two
// ways the Community edition grants it. An Enterprise installation can grant
// it through a custom role as well, and this does not see those - so the badge
// means "administrator, at least by these two routes" and never the reverse.
// Marking somebody who is not would be worse than missing somebody who is.
func (h Handlers) globalAdministrators() map[string]bool {
	administrators := map[string]bool{}
	if bootstrap := strings.ToLower(strings.TrimSpace(h.bootstrapAdminUser)); bootstrap != "" {
		administrators[bootstrap] = true
	}
	if bootstrap := strings.ToLower(strings.TrimSpace(h.bootstrapAdminEmail)); bootstrap != "" {
		administrators[bootstrap] = true
	}
	if h.keycloak == nil {
		return administrators
	}
	members, err := h.keycloak.ListRealmRoleMembers(globalAdminRole)
	if err != nil {
		// The list of accounts is what was asked for; a directory that will
		// not answer this costs the badge, not the page.
		log.Printf("users: administrators unavailable, listing without the marker: %v", err)
		return administrators
	}
	for _, member := range members {
		if member.Username != "" {
			administrators[strings.ToLower(member.Username)] = true
		}
		if member.Email != "" {
			administrators[strings.ToLower(member.Email)] = true
		}
	}
	return administrators
}
