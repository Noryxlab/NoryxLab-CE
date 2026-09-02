package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func platformCondition(summary string) healthAlert {
	return healthAlert{
		Scope: health.ScopePlatform, Severity: healthCritical,
		Source: "backup", Summary: summary,
	}
}

// A failed job belongs to whoever ran it, raises its own alert from jobs.go,
// and would otherwise reach the operator twice - drowning the conditions that
// cost the business everything under the ones that cost one person an
// afternoon.
func TestUserAlertsAreNeitherNotifiedNorRecorded(t *testing.T) {
	notifier, drain := captureWebhook(t)
	store := memory.NewHealthEventStore()
	handlers := Handlers{notifier: notifier, healthEventStore: store}

	handlers.applyHealth(map[string]healthAlert{}, []healthAlert{{
		Scope: health.ScopeUser, Severity: healthWarning,
		Source: "jobs", Summary: "3 job(s) failed in the last 24 hours",
	}})

	if sent := drain(0); len(sent) != 0 {
		t.Fatalf("a user alert reached the operator channel: %+v", sent)
	}
	events, _ := store.List(time.Now().Add(-time.Hour), 10)
	if len(events) != 0 {
		t.Fatalf("a user alert was recorded as a platform condition: %+v", events)
	}
}

func TestPlatformConditionsAreRecordedAsIntervals(t *testing.T) {
	notifier, _ := captureWebhook(t)
	store := memory.NewHealthEventStore()
	handlers := Handlers{notifier: notifier, healthEventStore: store}

	firing := map[string]healthAlert{}
	handlers.applyHealth(firing, []healthAlert{platformCondition("no backup for more than two days")})

	open, _ := store.Open()
	if len(open) != 1 || !open[0].Open() {
		t.Fatalf("the condition was not recorded as open: %+v", open)
	}

	handlers.applyHealth(firing, nil)

	open, _ = store.Open()
	if len(open) != 0 {
		t.Fatalf("the condition is still open after clearing: %+v", open)
	}
	all, _ := store.List(time.Now().Add(-time.Hour), 10)
	if len(all) != 1 || all[0].ResolvedAt == nil {
		t.Fatalf("want one closed interval, got %+v", all)
	}
}

// The watcher forgets everything when the process restarts. Without recovering
// the open conditions it re-announces each of them as new, so three
// deployments in a day would tell the operator the platform recovered and
// relapsed three times.
func TestARestartDoesNotReAnnounceAnOngoingCondition(t *testing.T) {
	notifier, drain := captureWebhook(t)
	store := memory.NewHealthEventStore()
	handlers := Handlers{notifier: notifier, healthEventStore: store}

	condition := platformCondition("the latest backup failed")
	handlers.applyHealth(map[string]healthAlert{}, []healthAlert{condition})

	// A new process: fresh in-memory state, same store.
	restarted := Handlers{notifier: notifier, healthEventStore: store}
	firing := restarted.openConditions()
	if len(firing) != 1 {
		t.Fatalf("the open condition was not recovered: %v", sortedAlertKeys(firing))
	}
	restarted.applyHealth(firing, []healthAlert{condition})

	if sent := drain(1); len(sent) != 1 {
		t.Fatalf("sent %d alerts across a restart, want 1: %+v", len(sent), sent)
	}
	all, _ := store.List(time.Now().Add(-time.Hour), 10)
	if len(all) != 1 {
		t.Fatalf("the restart forked a second interval: %+v", all)
	}
}

// When the source knows when the condition actually began - a backup that last
// succeeded three days ago - the history says so rather than claiming it
// started the moment we noticed.
func TestTheRecordedStartIsWhenTheConditionBeganNotWhenWeLooked(t *testing.T) {
	store := memory.NewHealthEventStore()
	notifier, _ := captureWebhook(t)
	handlers := Handlers{notifier: notifier, healthEventStore: store}

	began := time.Now().UTC().Add(-72 * time.Hour)
	alert := platformCondition("no backup for more than two days")
	alert.Since = &began

	handlers.applyHealth(map[string]healthAlert{}, []healthAlert{alert})

	open, _ := store.Open()
	if len(open) != 1 {
		t.Fatalf("no open condition: %+v", open)
	}
	if open[0].RaisedAt.Sub(began).Abs() > time.Second {
		t.Fatalf("raisedAt = %v, want the condition's own start %v", open[0].RaisedAt, began)
	}
}

// An alert firing for four months is the most important row in the table.
func TestPurgeKeepsOpenConditionsHoweverOld(t *testing.T) {
	store := memory.NewHealthEventStore()
	old := time.Now().UTC().AddDate(0, 0, -400)

	_ = store.Raise(health.Event{Key: "backup|old open", Source: "backup", Summary: "old open", RaisedAt: old})
	_ = store.Raise(health.Event{Key: "backup|old closed", Source: "backup", Summary: "old closed", RaisedAt: old})
	_ = store.Resolve("backup|old closed", old.Add(time.Hour))

	removed, err := store.Purge(time.Now().UTC().AddDate(0, 0, -180))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("purged %d, want 1", removed)
	}
	open, _ := store.Open()
	if len(open) != 1 || open[0].Summary != "old open" {
		t.Fatalf("the open condition was purged: %+v", open)
	}
}
