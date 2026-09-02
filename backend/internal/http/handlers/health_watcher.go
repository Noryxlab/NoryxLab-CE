package handlers

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
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
	notifying := h.notifier != nil && h.notifier.Enabled()
	recording := h.healthEventStore != nil

	if !notifying && !recording {
		log.Printf("health watcher not started: no alert webhook and no event store")
		return
	}
	if !notifying {
		// Said out loud: an operator who believes alerting is on and has never
		// configured a webhook is in exactly the position this closes. The
		// history is still written, so the record survives the missing channel.
		log.Printf("health watcher started (every %s); no alert webhook configured, recording history only",
			healthWatcherInterval)
	} else {
		log.Printf("health watcher started (every %s)", healthWatcherInterval)
	}

	go func() {
		ticker := time.NewTicker(healthWatcherInterval)
		defer ticker.Stop()

		// What was already open before this process started. Without it a
		// restart forgets everything, re-announces every current condition as
		// new, and forks a second interval in the history - so three
		// deployments in a day would claim the platform recovered and relapsed
		// three times.
		firing := h.openConditions()

		// The first sweep runs immediately, so a platform that starts in a bad
		// state says so at once rather than a quarter of an hour later.
		h.sweepHealth(firing)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.sweepHealth(firing)
				h.purgeHealthHistory()
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
		// Only platform conditions. A failed job belongs to whoever ran it and
		// already raises its own alert from jobs.go, so carrying it here sent
		// the operator a duplicate of somebody else's problem - and on a busy
		// platform, drowned "no backup for two days" in them.
		if alert.Scope != health.ScopePlatform {
			continue
		}
		current[healthAlertKey(alert)] = alert
	}

	for key, alert := range current {
		if _, already := firing[key]; already {
			continue
		}
		firing[key] = alert
		h.recordRaised(key, alert)
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
		h.recordResolved(key)
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

// healthHistoryRetention bounds the table. Open conditions are never purged
// however old: an alert that has been firing for four months is the most
// important row in it.
const healthHistoryRetention = 180 * 24 * time.Hour

func (h Handlers) purgeHealthHistory() {
	if h.healthEventStore == nil {
		return
	}
	removed, err := h.healthEventStore.Purge(time.Now().UTC().Add(-healthHistoryRetention))
	if err != nil {
		log.Printf("could not purge health history: %v", err)
		return
	}
	if removed > 0 {
		log.Printf("purged %d resolved health event(s) older than %s", removed, healthHistoryRetention)
	}
}

// openConditions rebuilds what the previous process had already announced.
func (h Handlers) openConditions() map[string]healthAlert {
	firing := map[string]healthAlert{}
	if h.healthEventStore == nil {
		return firing
	}
	events, err := h.healthEventStore.Open()
	if err != nil {
		log.Printf("could not read open health conditions: %v", err)
		return firing
	}
	for _, event := range events {
		firing[event.Key] = healthAlert{
			Scope:    health.ScopePlatform,
			Severity: healthSeverity(event.Severity),
			Source:   event.Source,
			Summary:  event.Summary,
			Detail:   event.Detail,
		}
	}
	return firing
}

func (h Handlers) recordRaised(key string, alert healthAlert) {
	if h.healthEventStore == nil {
		return
	}
	raisedAt := time.Now().UTC()
	if alert.Since != nil {
		// When the source knows when the condition actually started - a backup
		// that last succeeded three days ago - the history says so rather than
		// claiming it began the moment we noticed.
		raisedAt = alert.Since.UTC()
	}
	err := h.healthEventStore.Raise(health.Event{
		Key:      key,
		Source:   alert.Source,
		Severity: health.Severity(alert.Severity),
		Summary:  alert.Summary,
		Detail:   alert.Detail,
		RaisedAt: raisedAt,
	})
	if err != nil {
		log.Printf("could not record health condition %q: %v", key, err)
	}
}

func (h Handlers) recordResolved(key string) {
	if h.healthEventStore == nil {
		return
	}
	if err := h.healthEventStore.Resolve(key, time.Now().UTC()); err != nil {
		log.Printf("could not close health condition %q: %v", key, err)
	}
}
