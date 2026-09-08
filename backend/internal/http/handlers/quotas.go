package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/quota"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/status"
)

// What a project may run at once, and what it is running now.
//
// The check happens at launch, against what is already running, and a refusal
// names the dimension, the ceiling and what is in use. "Quota exceeded" makes
// somebody open a terminal; "vCPU: 6 in use, 2 requested, limit 6" tells them
// to stop a workspace.

// projectUsage adds up what the project holds right now. Only workloads that
// are running or on their way count: a workspace that failed last week is not
// occupying a node.
func (h Handlers) projectUsage(projectID string) (quota.Usage, error) {
	usage := quota.Usage{}
	projectID = strings.TrimSpace(projectID)

	workspaces, err := h.workspaceStore.List()
	if err != nil {
		return usage, err
	}
	for _, item := range workspaces {
		if item.ProjectID != projectID || !occupiesTheCluster(item.Status) {
			continue
		}
		usage.Workspaces++
		usage.VCPU += cpuCores(item.CPU)
		usage.MemoryGiB += memoryBytes(item.Memory) / (1 << 30)
	}

	if h.jobStore != nil {
		jobs, err := h.jobStore.List()
		if err != nil {
			return usage, err
		}
		for _, item := range jobs {
			if item.ProjectID != projectID || !occupiesTheCluster(item.Status) {
				continue
			}
			usage.Jobs++
			// A job carries the tier it was launched with; an older record
			// that predates that column counts as one job and no CPU rather
			// than blocking a launch on a number nobody stored.
			if tier, found := h.resolveHardwareTier(item.HardwareTier); found && strings.TrimSpace(item.HardwareTier) != "" {
				usage.VCPU += cpuCores(tier.CPULimit)
				usage.MemoryGiB += memoryBytes(tier.MemoryLimit) / (1 << 30)
			}
		}
	}
	// Apps hold a pod for as long as they are published, and counted for
	// nothing: the hardware tier they are launched with was never stored, so a
	// Streamlit application was free and only an application was.
	if h.appStore != nil {
		apps, err := h.appStore.List()
		if err != nil {
			return usage, err
		}
		for _, item := range apps {
			if item.ProjectID != projectID || !occupiesTheCluster(item.Status) {
				continue
			}
			usage.Apps++
			if tier, found := h.resolveHardwareTier(item.HardwareTier); found && strings.TrimSpace(item.HardwareTier) != "" {
				usage.VCPU += cpuCores(tier.CPULimit)
				usage.MemoryGiB += memoryBytes(tier.MemoryLimit) / (1 << 30)
			}
		}
	}
	return usage, nil
}

// occupiesTheCluster says whether a workload in this state is holding
// resources. It reads the platform's own vocabulary rather than a second list
// of strings kept here - the two would drift, and that is exactly how
// "launching" once froze a screen.
func occupiesTheCluster(value string) bool {
	kind, declared := status.Of(strings.ToLower(strings.TrimSpace(value)))
	if !declared {
		// An unknown status is treated as running: refusing to count it would
		// let an unnamed state slip past every limit.
		return true
	}
	return kind == status.KindPending || kind == status.KindSuccess
}

// withinProjectQuota is the gate every launch passes through.
func (h Handlers) withinProjectQuota(w http.ResponseWriter, projectID, kind string, tier hardwareTierLimits) bool {
	if h.quotaStore == nil {
		return true
	}
	limits, found, err := h.quotaStore.Get(projectID)
	if err != nil {
		// A quota store that cannot answer must not become a platform that
		// refuses everything: the limit is a guard, not a dependency.
		return true
	}
	if !found || limits.Empty() {
		return true
	}
	usage, err := h.projectUsage(projectID)
	if err != nil {
		return true
	}
	refusal := limits.Allows(usage, cpuCores(tier.CPULimit), memoryBytes(tier.MemoryLimit)/(1<<30), kind)
	if refusal == nil {
		return true
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":  refusal.Error(),
		"code":   "project_quota_reached",
		"quota":  limits,
		"usage":  usage,
		"detail": refusal,
	})
	return false
}

// hardwareTierLimits is the part of a tier a quota cares about.
type hardwareTierLimits struct {
	CPULimit    string
	MemoryLimit string
}

func (h Handlers) GetProjectQuota(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectMember(w, projectID, identity.UserID(), "project quota") {
		return
	}
	limits := quota.Quota{ProjectID: projectID}
	if h.quotaStore != nil {
		if stored, found, err := h.quotaStore.Get(projectID); err == nil && found {
			limits = stored
		}
	}
	usage, err := h.projectUsage(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to measure project usage"})
		return
	}
	// Both together, always: a limit without the current usage tells a user
	// nothing about whether they can start anything.
	writeJSON(w, http.StatusOK, map[string]any{"quota": limits, "usage": usage, "limited": !limits.Empty()})
}

func (h Handlers) SetProjectQuota(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	if h.quotaStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "quotas are not available on this installation"})
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if exists, err := h.projectExists(projectID); err != nil || !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	var limits quota.Quota
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&limits) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a valid quota is required"})
		return
	}
	limits.ProjectID = projectID
	if limits.MaxVCPU < 0 || limits.MaxMemoryGiB < 0 || limits.MaxWorkspaces < 0 || limits.MaxJobs < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a limit cannot be negative; use 0 for no limit"})
		return
	}
	if err := h.quotaStore.Set(limits); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save the quota"})
		return
	}
	h.emitAudit(r, identity.UserID(), "project.quota.set", "project", projectID, projectID, "success", "", map[string]any{
		"maxVcpu": limits.MaxVCPU, "maxMemoryGib": limits.MaxMemoryGiB,
		"maxWorkspaces": limits.MaxWorkspaces, "maxJobs": limits.MaxJobs,
	})
	usage, _ := h.projectUsage(projectID)
	// Answering with the usage as well: an administrator who has just set a
	// limit below what is already running should see that immediately, not
	// discover it from a user whose launch was refused.
	writeJSON(w, http.StatusOK, map[string]any{"quota": limits, "usage": usage, "limited": !limits.Empty()})
}
