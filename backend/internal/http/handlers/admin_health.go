package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"

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
	// Scope says whose problem this is. A failed job belongs to the person who
	// ran it and has context on their own screen; "no backup for two days"
	// belongs to whoever keeps the platform alive. Interleaving them buries
	// the condition that costs the business everything under the ones that
	// cost one person an afternoon.
	Scope   health.Scope `json:"scope"`
	Source  string       `json:"source"`
	Summary string       `json:"summary"`
	Detail  string       `json:"detail,omitempty"`
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

	writeJSON(w, http.StatusOK, h.platformHealth())
}

// platformHealth computes the report. Shared with the watcher so the screen
// and the alert can never disagree about the platform's state - two code paths
// answering the same question eventually give two answers, and it is the
// reassuring one that gets believed.
func (h Handlers) platformHealth() healthReport {
	alerts := []healthAlert{}
	alerts = append(alerts, h.identityAlerts()...)
	alerts = append(alerts, h.certificateAlerts()...)
	alerts = append(alerts, h.backupAlerts()...)
	alerts = append(alerts, h.deploymentAlerts()...)
	alerts = append(alerts, h.workspaceLifetimeAlerts()...)
	alerts = append(alerts, h.jobFailureAlerts()...)

	// Most severe first: an operator reads the top of the list.
	rank := map[healthSeverity]int{healthCritical: 0, healthWarning: 1, healthInfo: 2}
	sort.SliceStable(alerts, func(i, j int) bool {
		return rank[alerts[i].Severity] < rank[alerts[j].Severity]
	})

	return healthReport{
		GeneratedAt: time.Now().UTC(),
		Status:      overallHealth(alerts),
		Alerts:      alerts,
	}
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

// jobFailureAlerts reports failures from the last day. A webhook alert is a
// moment in time and is easily missed; the health report is what an operator
// can still consult afterwards.
func (h Handlers) jobFailureAlerts() []healthAlert {
	if h.jobStore == nil {
		return nil
	}
	records, err := h.jobStore.List()
	if err != nil {
		return nil
	}
	since := time.Now().UTC().Add(-24 * time.Hour)
	names := []string{}
	var latest *time.Time
	for _, record := range records {
		if !isFailedJobStatus(record.Status) {
			continue
		}
		at := record.CreatedAt
		if record.CompletedAt != nil {
			at = *record.CompletedAt
		}
		if at.Before(since) {
			continue
		}
		names = append(names, jobDisplayName(record))
		if latest == nil || at.After(*latest) {
			moment := at
			latest = &moment
		}
	}
	if len(names) == 0 {
		return nil
	}
	return []healthAlert{{
		Scope:    health.ScopeUser,
		Severity: healthWarning,
		Source:   "jobs",
		Summary:  strconv.Itoa(len(names)) + " job(s) failed in the last 24 hours",
		Detail:   strings.Join(names, ", "),
		Since:    latest,
		Action:   "activity",
	}}
}

func (h Handlers) deploymentAlerts() []healthAlert {
	inspector, ok := h.runtime.(noryxruntime.Inspector)
	if !ok || inspector == nil {
		return nil
	}
	deployments, err := inspector.ListDeployments()
	if err != nil {
		return []healthAlert{{
			Scope:    health.ScopePlatform,
			Severity: healthWarning,
			Source:   "runtime",
			Summary:  "deployment status is unreadable",
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
			Scope:    health.ScopePlatform,
			Severity: severity,
			Source:   "runtime",
			Summary:  "component unavailable: " + deployment.Name,
			Detail:   strconv.Itoa(deployment.ReadyReplicas) + "/" + strconv.Itoa(deployment.Replicas) + " replicas ready",
			Action:   "overview",
		})
	}
	return alerts
}

// workspaceLifetimeAlerts reports workspaces the reaper should have reclaimed
// but has not, which means teardown is failing and capacity is leaking.
func (h Handlers) workspaceLifetimeAlerts() []healthAlert {
	maxLifetime := h.currentWorkspaceLifetime()
	if h.workspaceStore == nil || maxLifetime <= 0 {
		return nil
	}
	records, err := h.workspaceStore.List()
	if err != nil {
		return nil
	}
	// Generous margin over the sweep interval: a workspace one tick past its
	// deadline is not a problem, one an hour past it is.
	deadline := time.Now().UTC().Add(-maxLifetime - time.Hour)
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
		Scope:    health.ScopePlatform,
		Severity: healthWarning,
		Source:   "workspaces",
		Summary:  strconv.Itoa(len(stale)) + " workspace(s) exceed their maximum lifetime without having been stopped",
		Detail:   strings.Join(stale, ", "),
		Action:   "activity",
	}}
}

// healthHistoryDefaultDays bounds the default window. Ninety days of platform
// conditions is a few dozen rows on a healthy installation.
const (
	healthHistoryDefaultDays = 30
	healthHistoryMaxDays     = 365
	healthHistoryMaxItems    = 500
)

// GetPlatformHealthHistory answers "what has this platform been complaining
// about, and did it resolve".
//
// The health screen shows the present, and a webhook is fire-and-forget: if
// nobody was in the channel, the event is gone. Three nights without a backup
// left no consultable trace anywhere, which is the gap this closes.
func (h Handlers) GetPlatformHealthHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "overview"); !ok {
		return
	}
	if h.healthEventStore == nil {
		// Distinguished from an empty history on purpose: "nothing recorded"
		// and "nothing happened" are different answers, and conflating them is
		// how a silent platform reads as a healthy one.
		writeJSON(w, http.StatusOK, map[string]any{
			"items": []health.Event{}, "recording": false,
		})
		return
	}

	days := healthHistoryDefaultDays
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > healthHistoryMaxDays {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "days must be a positive integer of at most " + strconv.Itoa(healthHistoryMaxDays),
			})
			return
		}
		days = parsed
	}

	since := time.Now().UTC().AddDate(0, 0, -days)
	events, err := h.healthEventStore.List(since, healthHistoryMaxItems)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read health history"})
		return
	}
	if events == nil {
		events = []health.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": events, "since": since, "recording": true,
	})
}
