// Package notify delivers platform alerts to an operator.
//
// Its absence is why the manifest-only backup reported success for months and
// why the 2026-05-04 frontend freeze was found by a person noticing a broken
// page. The platform already knew both: the backup wrote its own warning into
// every report, and the validation suite files reports nobody opens. What was
// missing is the loop closing on its own (ADR-034).
//
// A generic webhook is the transport. The payload carries a `text` field, so a
// Slack, Teams or Mattermost incoming webhook renders it directly, alongside
// structured fields for any other consumer.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Alert is one operator-facing event.
type Alert struct {
	Severity Severity       `json:"severity"`
	Event    string         `json:"event"`
	Summary  string         `json:"summary"`
	Details  map[string]any `json:"details,omitempty"`
	// Text renders the alert in chat webhooks that expect this field.
	Text        string    `json:"text"`
	Instance    string    `json:"instance,omitempty"`
	OccurredAt  time.Time `json:"occurredAt"`
	SchemaVersi string    `json:"schemaVersion"`
}

// Notifier delivers alerts. A zero Notifier is valid and discards them, so
// callers never have to guard on configuration.
type Notifier struct {
	webhookURL string
	instance   string
	// resolve, when set, supplies the destination at send time so an
	// administrator changing it takes effect without a restart.
	resolve func() (webhookURL string, instance string)
	// format is resolved per send, like the destination: an operator changing
	// it must not have to restart the platform to see the effect.
	format func() string
	client *http.Client
}

// New returns a Notifier posting to webhookURL. An empty URL disables
// delivery: alerting is opt-in, and a platform without a configured channel
// must keep working.
func New(webhookURL, instance string) *Notifier {
	url := strings.TrimSpace(webhookURL)
	if url == "" {
		log.Printf("alerting disabled (no webhook configured)")
		return &Notifier{}
	}
	log.Printf("alerting enabled")
	return &Notifier{
		webhookURL: url,
		instance:   strings.TrimSpace(instance),
		// Short: an alert that blocks a request for a minute is a second
		// outage on top of the one being reported.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewDynamic resolves its destination on every send, so the webhook can be
// changed from the administration interface rather than by redeploying.
func NewDynamic(resolve func() (string, string)) *Notifier {
	return &Notifier{
		resolve: resolve,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// WithFormat chooses how the alert is written on the wire.
//
// A format, deliberately, and not an integration. JSON carries the whole alert
// and is what Slack, Teams and anything reading a field expect; plain text is
// what a receiver that shows the body as-is needs, such as a self-hosted ntfy.
// Adding one connector per vendor is how a product ends up maintaining eight of
// them, so there are two shapes and a ten-line relay bridges anything else.
func (n *Notifier) WithFormat(format func() string) *Notifier {
	if n != nil {
		n.format = format
	}
	return n
}

const (
	FormatJSON = "json"
	FormatText = "text"
)

func (n *Notifier) wireFormat() string {
	if n == nil || n.format == nil {
		return FormatJSON
	}
	if strings.EqualFold(strings.TrimSpace(n.format()), FormatText) {
		return FormatText
	}
	return FormatJSON
}

func (n *Notifier) destination() (string, string) {
	if n == nil {
		return "", ""
	}
	if n.resolve != nil {
		url, instance := n.resolve()
		return strings.TrimSpace(url), strings.TrimSpace(instance)
	}
	return n.webhookURL, n.instance
}

func (n *Notifier) Enabled() bool {
	if n == nil {
		return false
	}
	url, _ := n.destination()
	return url != ""
}

// Send delivers an alert. Failures are logged and swallowed: a broken
// alerting channel must never take down the operation that raised the alert.
func (n *Notifier) Send(ctx context.Context, alert Alert) {
	webhookURL, instance := n.destination()
	if webhookURL == "" {
		return
	}
	if alert.OccurredAt.IsZero() {
		alert.OccurredAt = time.Now().UTC()
	}
	if alert.Text == "" {
		alert.Text = alert.render(instance)
	}
	alert.Instance = instance
	alert.SchemaVersi = "noryx-alert-v1"

	var body []byte
	contentType := "application/json"
	if n.wireFormat() == FormatText {
		body = []byte(alert.Text)
		contentType = "text/plain; charset=utf-8"
	} else {
		encoded, err := json.Marshal(alert)
		if err != nil {
			log.Printf("alert %q could not be encoded: %v", alert.Event, err)
			return
		}
		body = encoded
	}

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("alert %q could not be prepared: %v", alert.Event, err)
		return
	}
	req.Header.Set("Content-Type", contentType)

	// Title and priority as headers. These are ntfy's, and any other receiver
	// ignores them - which is why they cost nothing to send and are not an
	// integration. Without a priority a critical alert arrives as quietly as an
	// informational one, and a phone that does not ring for an expired
	// certificate is a phone that told nobody.
	req.Header.Set("X-Title", alertTitle(alert, instance))
	req.Header.Set("X-Priority", alertPriority(alert.Severity))

	resp, err := n.client.Do(req)
	if err != nil {
		log.Printf("alert %q could not be delivered: %v", alert.Event, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("alert %q rejected by the webhook: HTTP %d", alert.Event, resp.StatusCode)
		return
	}
	log.Printf("alert %q delivered (%s)", alert.Event, alert.Severity)
}

// SendAsync delivers without blocking the caller. Used on request paths, where
// the operator's alert must not add latency to a user's action.
func (n *Notifier) SendAsync(alert Alert) {
	if !n.Enabled() {
		return
	}
	go n.Send(context.Background(), alert)
}

// alertTitle is what a phone shows on the lock screen, so it names the
// installation: an operator running several must not have to open the message
// to learn which one is unwell.
func alertTitle(alert Alert, instance string) string {
	title := "Noryx"
	if instance != "" {
		title += " " + instance
	}
	switch alert.Severity {
	case SeverityCritical:
		return title + " - critical"
	case SeverityWarning:
		return title + " - warning"
	default:
		return title
	}
}

// alertPriority maps severity onto ntfy's 1-5 scale. Critical is 5, which
// bypasses a phone's quiet hours; a warning is 4; anything else is the default.
func alertPriority(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "5"
	case SeverityWarning:
		return "4"
	default:
		return "3"
	}
}

func (a Alert) render(instance string) string {
	icon := map[Severity]string{
		SeverityInfo:     "i",
		SeverityWarning:  "!",
		SeverityCritical: "!!",
	}[a.Severity]

	var b strings.Builder
	fmt.Fprintf(&b, "[%s] Noryx", icon)
	if instance != "" {
		fmt.Fprintf(&b, " %s", instance)
	}
	fmt.Fprintf(&b, " - %s", a.Summary)
	if len(a.Details) > 0 {
		keys := make([]string, 0, len(a.Details))
		for key := range a.Details {
			keys = append(keys, key)
		}
		// Stable order: an alert that reshuffles its fields between two
		// deliveries is hard to read and impossible to diff.
		sortStrings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "\n  %s: %v", key, a.Details[key])
		}
	}
	return b.String()
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
