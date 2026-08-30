package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Platform health.
//
// The notifier delivers alerts to a webhook, which assumes an operator already
// runs an alert collector. Most installations do not, and the platform then
// knows it is unwell while telling nobody - the failure this whole thread
// started from, where a backup reported success for months (ADR-034).
//
// This endpoint answers the same question from inside the product: what is
// wrong right now. It computes from live state rather than from a stored alert
// log, so a condition that resolves itself disappears on its own and nothing
// has to be acknowledged or cleaned up.

type healthSeverity string

const (
	healthCritical healthSeverity = "critical"
	healthWarning  healthSeverity = "warning"
	healthInfo     healthSeverity = "info"
)

type healthAlert struct {
	Severity healthSeverity `json:"severity"`
	Source   string         `json:"source"`
	Summary  string         `json:"summary"`
	Detail   string         `json:"detail,omitempty"`
	// Since is when the condition was last observed to start, when known.
	Since *time.Time `json:"since,omitempty"`
	// Action names the screen an operator should open, so the UI can link to it.
	Action string `json:"action,omitempty"`
}

type healthReport struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	Status      string        `json:"status"`
	Alerts      []healthAlert `json:"alerts"`
}

// GetPlatformHealth reports the conditions an operator should act on.
func (h Handlers) GetPlatformHealth(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "overview"); !ok {
		return
	}

	alerts := []healthAlert{}
	alerts = append(alerts, h.backupAlerts()...)
	alerts = append(alerts, h.deploymentAlerts()...)
	alerts = append(alerts, h.workspaceLifetimeAlerts()...)

	// Most severe first: an operator reads the top of the list.
	rank := map[healthSeverity]int{healthCritical: 0, healthWarning: 1, healthInfo: 2}
	sort.SliceStable(alerts, func(i, j int) bool {
		return rank[alerts[i].Severity] < rank[alerts[j].Severity]
	})

	writeJSON(w, http.StatusOK, healthReport{
		GeneratedAt: time.Now().UTC(),
		Status:      overallHealth(alerts),
		Alerts:      alerts,
	})
}

func overallHealth(alerts []healthAlert) string {
	for _, alert := range alerts {
		if alert.Severity == healthCritical {
			return "critical"
		}
	}
	for _, alert := range alerts {
		if alert.Severity == healthWarning {
			return "degraded"
		}
	}
	return "healthy"
}

// backupAlerts reports on the most recent run only. Older failures are history;
// what matters is whether the platform is protected right now.
func (h Handlers) backupAlerts() []healthAlert {
	// No store means the edition does not ship backups at all; an empty store
	// means none has run yet, which is reported below rather than here.
	if h.backupRunStore == nil {
		return nil
	}
	runs, err := h.backupRunStore.List()
	if err != nil || len(runs) == 0 {
		return nil
	}
	latest := runs[0]
	for _, run := range runs {
		if run.StartedAt.After(latest.StartedAt) {
			latest = run
		}
	}
	switch {
	case latest.Status == backupStatusFailed:
		return []healthAlert{{
			Severity: healthCritical,
			Source:   "backup",
			Summary:  "la derniere sauvegarde a echoue",
			Detail:   latest.Error,
			Since:    &latest.StartedAt,
			Action:   "backups",
		}}
	case latest.Status == backupStatusDegraded:
		return []healthAlert{{
			Severity: healthWarning,
			Source:   "backup",
			Summary:  "la derniere sauvegarde est incomplete : elle ne permet pas une restauration complete",
			Detail:   latest.Error,
			Since:    &latest.StartedAt,
			Action:   "backups",
		}}
	}
	// A backup that has not run for more than two days is as good as absent.
	if time.Since(latest.StartedAt) > 48*time.Hour {
		return []healthAlert{{
			Severity: healthWarning,
			Source:   "backup",
			Summary:  "aucune sauvegarde depuis plus de deux jours",
			Since:    &latest.StartedAt,
			Action:   "backups",
		}}
	}
	return nil
}

func (h Handlers) deploymentAlerts() []healthAlert {
	inspector, ok := h.runtime.(noryxruntime.Inspector)
	if !ok || inspector == nil {
		return nil
	}
	deployments, err := inspector.ListDeployments()
	if err != nil {
		return []healthAlert{{
			Severity: healthWarning,
			Source:   "runtime",
			Summary:  "l'etat des deploiements est illisible",
			Detail:   err.Error(),
		}}
	}
	alerts := []healthAlert{}
	for _, deployment := range deployments {
		if deployment.Replicas == 0 || deployment.ReadyReplicas >= deployment.Replicas {
			continue
		}
		severity := healthWarning
		if deployment.ReadyReplicas == 0 {
			// No replica ready means the component is down, not degraded.
			severity = healthCritical
		}
		alerts = append(alerts, healthAlert{
			Severity: severity,
			Source:   "runtime",
			Summary:  "composant indisponible : " + deployment.Name,
			Detail:   strconv.Itoa(deployment.ReadyReplicas) + "/" + strconv.Itoa(deployment.Replicas) + " replicas prets",
			Action:   "overview",
		})
	}
	return alerts
}

// workspaceLifetimeAlerts reports workspaces the reaper should have reclaimed
// but has not, which means teardown is failing and capacity is leaking.
func (h Handlers) workspaceLifetimeAlerts() []healthAlert {
	if h.workspaceStore == nil || h.workspaceMaxLifetime <= 0 {
		return nil
	}
	records, err := h.workspaceStore.List()
	if err != nil {
		return nil
	}
	// Generous margin over the sweep interval: a workspace one tick past its
	// deadline is not a problem, one an hour past it is.
	deadline := time.Now().UTC().Add(-h.workspaceMaxLifetime - time.Hour)
	stale := []string{}
	for _, record := range records {
		if record.CreatedAt.Before(deadline) {
			stale = append(stale, record.Name)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	return []healthAlert{{
		Severity: healthWarning,
		Source:   "workspaces",
		Summary:  strconv.Itoa(len(stale)) + " workspace(s) depassent leur duree de vie sans avoir ete arretes",
		Detail:   strings.Join(stale, ", "),
		Action:   "activity",
	}}
}
