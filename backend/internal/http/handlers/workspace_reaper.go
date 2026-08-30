package handlers

import (
	"context"
	"log"
	"time"
)

// Workspace lifetime reaper.
//
// A workspace previously ran until somebody stopped it, which is the first
// cost driver of any datalab and the question a CIO asks first: how do I stop
// people leaving compute running all weekend (ADR-034). On a GPU-backed
// installation, as contemplated by ADR-033, it stops being a cost question and
// becomes a capacity one.
//
// This enforces a maximum lifetime, deliberately not called idle detection.
// Detecting idleness needs an activity signal from the workspace itself -
// kernel activity, or requests seen by an ingress - and none exists today.
// Naming an age limit "idle stop" would be the same overclaim this ADR exists
// to remove: a user told their workspace stops when idle would reasonably
// expect an active one to survive.
const (
	workspaceReaperInterval = 5 * time.Minute
	// Grace applied on top of the configured lifetime, so a workspace is never
	// reclaimed on the exact boundary of a clock skew between API and cluster.
	workspaceReaperGrace = 1 * time.Minute
)

// StartWorkspaceReaper runs the lifetime sweep until ctx is cancelled. A
// non-positive lifetime disables it entirely, which is the default.
func (h Handlers) StartWorkspaceReaper(ctx context.Context, maxLifetime time.Duration) {
	if maxLifetime <= 0 {
		log.Printf("workspace reaper disabled (no maximum lifetime configured)")
		return
	}
	log.Printf("workspace reaper enabled: workspaces are stopped after %s", maxLifetime)

	go func() {
		ticker := time.NewTicker(workspaceReaperInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.reapExpiredWorkspaces(maxLifetime)
			}
		}
	}()
}

func (h Handlers) reapExpiredWorkspaces(maxLifetime time.Duration) {
	if h.workspaceStore == nil {
		return
	}
	records, err := h.workspaceStore.List()
	if err != nil {
		log.Printf("workspace reaper: cannot list workspaces: %v", err)
		return
	}

	deadline := time.Now().UTC().Add(-maxLifetime - workspaceReaperGrace)
	for _, record := range records {
		if record.CreatedAt.After(deadline) {
			continue
		}
		age := time.Since(record.CreatedAt).Truncate(time.Minute)
		if err := h.deleteWorkspaceResources(record); err != nil {
			// Left in place deliberately: the next sweep retries, and a
			// workspace that cannot be torn down must not vanish from the
			// store while its pod is still running.
			log.Printf("workspace reaper: cannot tear down %s (age %s): %v", record.ID, age, err)
			continue
		}
		if err := h.workspaceStore.Delete(record.ID); err != nil {
			log.Printf("workspace reaper: cannot delete record %s: %v", record.ID, err)
			continue
		}
		log.Printf("workspace reaper: stopped %s (%s) after %s", record.ID, record.Name, age)
		h.emitSystemAudit("workspace.reap", "workspace", record.ID, record.ProjectID, map[string]any{
			"name":        record.Name,
			"podName":     record.PodName,
			"ageMinutes":  int(age.Minutes()),
			"maxLifetime": maxLifetime.String(),
			"reason":      "maximum lifetime reached",
		})
	}
}
