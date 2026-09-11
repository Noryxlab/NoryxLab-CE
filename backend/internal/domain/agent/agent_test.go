package agent

import (
	"testing"
	"time"
)

// A mission cannot grant a power. The text is instructions to a model; the
// actions are a permission the platform enforces, and the two must never be
// the same field - the person writing the mission is also the person the
// permission protects.
func TestAnUnknownActionIsNeverStored(t *testing.T) {
	granted := NormaliseActions([]string{"restart_app", "delete_project", "", "RESTART_APP", "spend_money"})
	if len(granted) != 1 || granted[0] != ActionRestartApp {
		t.Fatalf("granted = %v, want only %q", granted, ActionRestartApp)
	}
}

// An agent nobody granted an action must not take one.
func TestAnAgentWithoutActionsMayNothing(t *testing.T) {
	watcher := New("amandine", "", "Clémence", "Surveille mes workspaces.", ScheduleHourly, nil)
	if watcher.May(ActionRestartApp) {
		t.Fatal("an agent with no granted action may act")
	}
	if len(watcher.Actions) != 0 {
		t.Fatalf("actions = %v, want none", watcher.Actions)
	}
}

// A schedule nobody recognises falls back to manual, which is the one that
// cannot surprise anybody. Defaulting to hourly would make a typo into a
// standing job.
func TestAnUnknownScheduleBecomesManual(t *testing.T) {
	for _, given := range []string{"", "every 5 minutes", "* * * * *", "HOURLY "} {
		got := NormaliseSchedule(given)
		if given == "HOURLY " {
			if got != ScheduleHourly {
				t.Fatalf("%q became %q", given, got)
			}
			continue
		}
		if got != ScheduleManual {
			t.Fatalf("%q became %q, want manual", given, got)
		}
	}
}

func TestWhenAnAgentIsDue(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	hourAgo := now.Add(-time.Hour)
	minuteAgo := now.Add(-time.Minute)

	hourly := New("amandine", "", "Clémence", "…", ScheduleHourly, nil)
	if !hourly.DueAt(now) {
		t.Fatal("an agent that has never run is due")
	}
	hourly.LastRunAt = &minuteAgo
	if hourly.DueAt(now) {
		t.Fatal("an hourly agent ran a minute ago and is not due")
	}
	hourly.LastRunAt = &hourAgo
	if !hourly.DueAt(now) {
		t.Fatal("an hourly agent that ran an hour ago is due")
	}

	// A disabled agent is never due. Somebody paused it on purpose, and a
	// scheduler that runs it anyway is the reason people delete features
	// instead of pausing them.
	hourly.Enabled = false
	if hourly.DueAt(now) {
		t.Fatal("a paused agent was scheduled")
	}

	manual := New("amandine", "", "Aymeric", "…", ScheduleManual, nil)
	if manual.DueAt(now) {
		t.Fatal("a manual agent is never due on its own")
	}
}
