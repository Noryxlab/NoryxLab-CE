package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
)

// captureWebhook collects what the notifier actually sends.
func captureWebhook(t *testing.T) (*notify.Notifier, func(want int) []notify.Alert) {
	t.Helper()
	var mu sync.Mutex
	var received []notify.Alert

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var alert notify.Alert
		_ = json.NewDecoder(r.Body).Decode(&alert)
		mu.Lock()
		received = append(received, alert)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	// Delivery is asynchronous by design - a slow webhook must not hold up the
	// sweep - so the drain waits for a count rather than a moment, and callers
	// assert on the set rather than the order. Alerts are self-describing;
	// requiring them to arrive in sequence would be testing the scheduler.
	return notify.New(server.URL, "test"), func(want int) []notify.Alert {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			count := len(received)
			mu.Unlock()
			if count >= want {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		return append([]notify.Alert(nil), received...)
	}
}

func eventsOf(alerts []notify.Alert) map[string]notify.Alert {
	out := map[string]notify.Alert{}
	for _, alert := range alerts {
		out[alert.Event] = alert
	}
	return out
}

func TestASweepAnnouncesANewConditionOnce(t *testing.T) {
	// The point of the watcher: a condition that appears at 03:00 is reported
	// then, not when somebody opens the screen. And repeating it every sweep
	// would teach the operator to filter the channel, which is the same as
	// having none.
	notifier, drain := captureWebhook(t)
	handlers := Handlers{notifier: notifier}

	since := time.Now().Add(-72 * time.Hour).UTC()
	condition := healthAlert{
		Severity: healthWarning, Source: "backup",
		Summary: "no backup for more than two days", Since: &since, Action: "backups",
	}

	firing := map[string]healthAlert{}
	handlers.applyHealth(firing, []healthAlert{condition})
	handlers.applyHealth(firing, []healthAlert{condition})

	sent := drain(1)
	if len(sent) != 1 {
		t.Fatalf("sent %d alerts, want exactly 1: %+v", len(sent), sent)
	}
	raised := sent[0]
	if raised.Event != "platform.health.raised" {
		t.Fatalf("event = %q", raised.Event)
	}
	if raised.Severity != notify.SeverityWarning {
		t.Fatalf("severity = %q, want warning", raised.Severity)
	}
	if raised.Details["screen"] != "backups" {
		t.Fatalf("the alert must name the screen to open: %+v", raised.Details)
	}
}

func TestRecoveryIsAnnouncedToo(t *testing.T) {
	// Without it an operator cannot tell a fixed problem from a forgotten one,
	// and the next silence is ambiguous.
	notifier, drain := captureWebhook(t)
	handlers := Handlers{notifier: notifier}

	condition := healthAlert{Severity: healthCritical, Source: "backup", Summary: "the latest backup failed"}
	firing := map[string]healthAlert{}
	handlers.applyHealth(firing, []healthAlert{condition})
	handlers.applyHealth(firing, nil)

	sent := drain(2)
	events := eventsOf(sent)
	if len(sent) != 2 {
		t.Fatalf("sent %d alerts, want 2: %+v", len(sent), sent)
	}
	if _, ok := events["platform.health.raised"]; !ok {
		t.Fatalf("no raise: %+v", sent)
	}
	cleared, ok := events["platform.health.cleared"]
	if !ok {
		t.Fatalf("no recovery: %+v", sent)
	}
	if cleared.Severity != notify.SeverityInfo {
		t.Fatalf("recovery severity = %q, want info", cleared.Severity)
	}
	if len(firing) != 0 {
		t.Fatalf("the condition is still tracked as firing: %v", sortedAlertKeys(firing))
	}
}

// A failing backup whose error text shifts slightly is the same ongoing
// condition. Keying on the detail would re-announce it on every sweep.
func TestAChangingDetailDoesNotReAnnounceTheSameCondition(t *testing.T) {
	notifier, drain := captureWebhook(t)
	handlers := Handlers{notifier: notifier}

	firing := map[string]healthAlert{}
	handlers.applyHealth(firing, []healthAlert{
		{Severity: healthCritical, Source: "backup", Summary: "the latest backup failed", Detail: "connection reset"},
	})
	handlers.applyHealth(firing, []healthAlert{
		{Severity: healthCritical, Source: "backup", Summary: "the latest backup failed", Detail: "i/o timeout"},
	})

	if sent := drain(1); len(sent) != 1 {
		t.Fatalf("sent %d alerts, want 1: %+v", len(sent), sent)
	}
}

func TestTheWatcherDoesNotStartWithoutAWebhook(t *testing.T) {
	// Starting it silently would let an operator believe alerting is on.
	handlers := Handlers{notifier: notify.New("", "test")}
	if handlers.notifier.Enabled() {
		t.Fatal("an unconfigured notifier reports itself as enabled")
	}
}
