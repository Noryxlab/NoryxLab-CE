package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
)

// One way out for anything that needs to tell somebody.
//
// The platform validator runs every night and records its result in the
// database. Nothing announced it, so the daily suite failed every night from
// at least 2026-09-23 to 2026-09-30 - eight nights, on one field the workspace
// endpoint had stopped accepting - and nobody could know without opening the
// screen. A check that fails silently for a week is not a check.
//
// Giving the validator its own mail account was the obvious fix and the wrong
// one: a second copy of a credential is a second thing to rotate and forget,
// which is already written beside the gateway's own alerting. So the component
// that noticed something says so here, and the platform's single notifier
// delivers it to whatever is configured - a webhook, a mailbox, or nothing,
// which it says out loud rather than pretending.

type componentAlertRequest struct {
	Event   string `json:"event"`
	Summary string `json:"summary"`
	// Severity is the caller's reading of its own condition. Unknown values
	// become a warning rather than being refused: an alert lost to a
	// vocabulary mismatch is the failure this endpoint exists to prevent.
	Severity string         `json:"severity"`
	Details  map[string]any `json:"details,omitempty"`
}

func (h Handlers) PostComponentAlert(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	var req componentAlertRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Event = strings.TrimSpace(req.Event)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Event == "" || req.Summary == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event and summary are required"})
		return
	}

	if h.notifier == nil || !h.notifier.Enabled() {
		// Refused rather than accepted and dropped. A component told the
		// platform something was wrong; answering 202 while discarding it
		// would leave two systems each believing the other was handling it.
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "no alert destination is configured on this installation, so this alert was not delivered",
			"code":  "no_alert_destination",
		})
		h.emitAudit(r, identity.UserID(), "alert.component", "alert", req.Event, "",
			"failure", "no_destination", map[string]any{"summary": req.Summary})
		return
	}

	h.notifier.SendAsync(notify.Alert{
		Severity: alertSeverity(req.Severity),
		Event:    req.Event,
		Summary:  req.Summary,
		Details:  req.Details,
	})
	h.emitAudit(r, identity.UserID(), "alert.component", "alert", req.Event, "",
		"success", "", map[string]any{"summary": req.Summary, "severity": req.Severity})
	w.WriteHeader(http.StatusAccepted)
}

// alertSeverity maps a caller's word onto the platform's, leaning toward
// being heard: an unrecognised severity is a warning, never silence.
func alertSeverity(value string) notify.Severity {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "error", "fatal":
		return notify.SeverityCritical
	case "info", "information", "notice":
		return notify.SeverityInfo
	default:
		return notify.SeverityWarning
	}
}
