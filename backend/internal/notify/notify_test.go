package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDisabledNotifierIsSafeAndSilent(t *testing.T) {
	for _, n := range []*Notifier{nil, New("", "test"), New("   ", "test")} {
		if n.Enabled() {
			t.Fatal("a notifier without a webhook must report itself disabled")
		}
		// Must not panic nor block.
		n.Send(t.Context(), Alert{Event: "x", Summary: "y"})
		n.SendAsync(Alert{Event: "x", Summary: "y"})
	}
}

func TestSendPostsRenderablePayload(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	New(server.URL, "datalab.example").Send(t.Context(), Alert{
		Severity: SeverityCritical,
		Event:    "backup.degraded",
		Summary:  "incomplete backup",
		Details:  map[string]any{"bucket": "noryx-backup", "bytes": 2348},
	})

	select {
	case body := <-received:
		var payload Alert
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("payload is not valid JSON: %v", err)
		}
		// Chat webhooks render this field; without it Slack and Teams show an
		// empty message.
		if payload.Text == "" {
			t.Fatal("payload must carry a text field for chat webhooks")
		}
		if !strings.Contains(payload.Text, "incomplete backup") {
			t.Fatalf("text must contain the summary, got %q", payload.Text)
		}
		if !strings.Contains(payload.Text, "datalab.example") {
			t.Fatalf("text must name the instance, got %q", payload.Text)
		}
		if payload.OccurredAt.IsZero() {
			t.Fatal("occurredAt must be stamped")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no request reached the webhook")
	}
}

// A rejecting or unreachable webhook must never break the operation that
// raised the alert.
func TestDeliveryFailureIsSwallowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	New(server.URL, "test").Send(t.Context(), Alert{Event: "e", Summary: "s"})

	unreachable := New("http://127.0.0.1:1/hook", "test")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	unreachable.Send(ctx, Alert{Event: "e", Summary: "s"})
}

func TestDetailsRenderInStableOrder(t *testing.T) {
	alert := Alert{
		Severity: SeverityWarning,
		Summary:  "s",
		Details:  map[string]any{"zebra": 1, "alpha": 2, "mid": 3},
	}
	first := alert.render("i")
	for range 20 {
		if alert.render("i") != first {
			t.Fatal("detail ordering must be stable between renders")
		}
	}
	if strings.Index(first, "alpha") > strings.Index(first, "zebra") {
		t.Fatal("details must render in sorted order")
	}
}
