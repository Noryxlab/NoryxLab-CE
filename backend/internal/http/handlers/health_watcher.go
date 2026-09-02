package handlers

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
)

// Health watcher.
//
// The platform health report already knew, for example, that no backup had run
// for more than two days. It was only ever computed while serving a request,
// so it knew it in private: the nightly backup stopped for three nights and
// said so on a screen nobody had open, while the only other trace was a log
// line in a pod that later restarted.
//
// A condition the platform can detect and does not report is worse than one it
// cannot detect, because it manufactures the belief that someone is watching
// (ADR-034).
const healthWatcherInterval = 15 * time.Minute

// StartHealthWatcher evaluates the health report on its own schedule and
// notifies when a condition appears or clears.
func (h Handlers) StartHealthWatcher(ctx context.Context) {
	if h.notifier == nil || !h.notifier.Enabled() {
		// Said out loud: an operator who believes alerting is on and has never
		// configured a webhook is in exactly the position this closes.
		log.Printf("health watcher not started: no alert webhook is configured")
		return
	}
	log.Printf("health watcher started (every %s)", healthWatcherInterval)

	go func() {
		ticker := time.NewTicker(healthWatcherInterval)
		defer ticker.Stop()

		// The first sweep runs immediately, so a platform that starts in a bad
		// state says so at once rather than a quarter of an hour later.
		firing := map[string]healthAlert{}
		h.sweepHealth(firing)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.sweepHealth(firing)
			}
		}
	}()
}

// sweepHealth compares the current report against what is already firing and
// notifies on the difference.
//
// Only transitions are sent. Repeating an unchanged alert every quarter of an
// hour is how an operator learns to filter the channel, and a filtered channel
// is an absent one.
func (h Handlers) sweepHealth(firing map[string]healthAlert) {
	h.applyHealth(firing, h.platformHealth().Alerts)
}

// applyHealth is the comparison itself, separated so it can be tested against
// a chosen set of conditions rather than against whatever the platform happens
// to be doing.
func (h Handlers) applyHealth(firing map[string]healthAlert, alerts []healthAlert) {
	current := map[string]healthAlert{}
	for _, alert := range alerts {
		current[healthAlertKey(alert)] = alert
	}

	for key, alert := range current {
		if _, already := firing[key]; already {
			continue
		}
		firing[key] = alert
		h.notifier.SendAsync(notify.Alert{
			Severity: healthSeverityToNotify(alert.Severity),
			Event:    "platform.health.raised",
			Summary:  alert.Summary,
			Details:  healthAlertDetails(alert),
		})
	}

	for key, alert := range firing {
		if _, still := current[key]; still {
			continue
		}
		delete(firing, key)
		// Recovery is sent too. Without it an operator cannot tell a fixed
		// problem from a forgotten one, and the next silence is ambiguous.
		h.notifier.SendAsync(notify.Alert{
			Severity: notify.SeverityInfo,
			Event:    "platform.health.cleared",
			Summary:  "resolved: " + alert.Summary,
			Details:  healthAlertDetails(alert),
		})
	}
}

// healthAlertKey identifies a condition across sweeps.
//
// Source and summary, never the detail: a failing backup whose error message
// changes slightly is the same ongoing condition, and keying on the detail
// would re-announce it on every sweep.
func healthAlertKey(alert healthAlert) string {
	return alert.Source + "|" + alert.Summary
}

func healthAlertDetails(alert healthAlert) map[string]any {
	details := map[string]any{"source": alert.Source}
	if strings.TrimSpace(alert.Detail) != "" {
		details["detail"] = alert.Detail
	}
	if alert.Since != nil {
		details["since"] = alert.Since.UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(alert.Action) != "" {
		details["screen"] = alert.Action
	}
	return details
}

func healthSeverityToNotify(severity healthSeverity) notify.Severity {
	switch severity {
	case healthCritical:
		return notify.SeverityCritical
	case healthWarning:
		return notify.SeverityWarning
	default:
		return notify.SeverityInfo
	}
}

// sortedAlertKeys exists for the tests, which need a stable order to assert on.
func sortedAlertKeys(alerts map[string]healthAlert) []string {
	keys := make([]string, 0, len(alerts))
	for key := range alerts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
