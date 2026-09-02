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

// The wire format is a format, not an integration: one JSON shape for anything
// that reads fields, one plain-text shape for anything that shows the body.
func TestTextFormatSendsTheRenderedAlertAsText(t *testing.T) {
	var contentType, title, priority, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		title = r.Header.Get("X-Title")
		priority = r.Header.Get("X-Priority")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDynamic(func() (string, string) { return server.URL, "premyom" }).
		WithFormat(func() string { return FormatText })
	notifier.Send(context.Background(), Alert{
		Severity: SeverityCritical,
		Event:    "platform.health.raised",
		Summary:  "the TLS certificate expires in less than a week",
		Details:  map[string]any{"source": "certificate"},
	})

	if !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("content type = %q, want text/plain", contentType)
	}
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		t.Fatalf("body is JSON in text mode: %q", body)
	}
	if !strings.Contains(body, "the TLS certificate expires in less than a week") {
		t.Fatalf("body does not carry the summary: %q", body)
	}
	// A phone must say which installation is unwell without opening the message.
	if !strings.Contains(title, "premyom") {
		t.Fatalf("title = %q, want the instance name in it", title)
	}
	// A critical alert that arrives as quietly as an informational one is a
	// phone that told nobody.
	if priority != "5" {
		t.Fatalf("priority = %q, want 5 for critical", priority)
	}
}

func TestJSONRemainsTheDefault(t *testing.T) {
	var contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	NewDynamic(func() (string, string) { return server.URL, "premyom" }).
		Send(context.Background(), Alert{Severity: SeverityInfo, Event: "e", Summary: "s"})

	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("content type = %q, want JSON by default", contentType)
	}
	if !strings.Contains(body, `"schemaVersion"`) {
		t.Fatalf("the JSON contract is not intact: %q", body)
	}
}
