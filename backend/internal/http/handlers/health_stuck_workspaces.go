package handlers

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// A workspace that never starts.
//
// Two people were onboarded one morning and neither could work: their pods sat
// in ContainerCreating for four hours because Longhorn had no schedulable
// space left, and the interface showed "request recorded" the whole time. The
// platform knew - Kubernetes had recorded the reason on the pod, twice a
// minute, for four hours - and nothing asked.
//
// This asks. It is deliberately about the symptom rather than about storage:
// a volume that cannot be placed, an image that cannot be pulled and a node
// that cannot be scheduled all end the same way, with somebody waiting in
// front of a screen that says nothing is wrong. The Kubernetes reason is
// carried through to the detail, because "insufficient storage" is what makes
// the difference between a fix and a guess.
const (
	// Long enough that a large image or a cold node is not an incident, short
	// enough that nobody spends a morning waiting.
	workspaceStartGrace = 10 * time.Minute
	// Beyond this it is not slow, it is stuck, and it will not resolve itself.
	workspaceStartLost = 45 * time.Minute
)

func (h Handlers) stuckWorkspaceAlerts() []healthAlert {
	if h.workspaceStore == nil || h.runtime == nil {
		return nil
	}
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		return nil
	}
	records, err := h.workspaceStore.List()
	if err != nil {
		return nil
	}

	now := time.Now().UTC()
	type stuck struct {
		name   string
		age    time.Duration
		reason string
	}
	var waiting []stuck
	worst := time.Duration(0)

	for _, record := range records {
		age := now.Sub(record.CreatedAt)
		if age < workspaceStartGrace || record.PodName == "" {
			continue
		}
		status, err := operator.GetPodStatus(record.PodName)
		if err != nil {
			// A pod the platform cannot read is not evidence of anything: the
			// workspace may have been torn down between the two calls.
			continue
		}
		if !strings.EqualFold(status.Phase, "Pending") {
			continue
		}
		if age > worst {
			worst = age
		}
		waiting = append(waiting, stuck{
			name:   firstNonEmpty(record.Name, record.ID),
			age:    age.Truncate(time.Minute),
			reason: h.whyPodIsWaiting(record.PodName, status),
		})
	}
	if len(waiting) == 0 {
		return nil
	}

	// Longest wait first: it is the one that has cost somebody the most, and
	// usually the one that explains the others.
	sort.Slice(waiting, func(i, j int) bool { return waiting[i].age > waiting[j].age })
	details := make([]string, 0, len(waiting))
	for _, item := range waiting {
		line := item.name + " (" + item.age.String() + ")"
		if item.reason != "" {
			line += ": " + item.reason
		}
		details = append(details, line)
	}

	severity := healthWarning
	if worst > workspaceStartLost {
		severity = healthCritical
	}
	return []healthAlert{{
		Scope:    health.ScopePlatform,
		Severity: severity,
		Source:   "workspaces",
		Summary: strconv.Itoa(len(waiting)) + " workspace(s) have not started, the oldest for " +
			worst.Truncate(time.Minute).String(),
		Detail: strings.Join(details, " · "),
		Action: "activity",
	}}
}

// whyPodIsWaiting answers in Kubernetes' own words.
//
// The pod's own message when it has one, otherwise the most recent warning
// recorded against it. Not translated: the operator who reads this is going to
// search for the exact string, and a friendlier paraphrase would be a string
// that matches nothing.
func (h Handlers) whyPodIsWaiting(podName string, status noryxruntime.PodStatus) string {
	if message := strings.TrimSpace(status.Message); message != "" {
		return trimReason(message)
	}
	reader, ok := h.runtime.(noryxruntime.PodEventReader)
	if !ok {
		return strings.TrimSpace(status.Reason)
	}
	events, err := reader.GetPodEvents(podName)
	if err != nil {
		return strings.TrimSpace(status.Reason)
	}
	latest := ""
	var at time.Time
	for _, event := range events {
		if !strings.EqualFold(event.Type, "Warning") {
			continue
		}
		if latest == "" || event.At.After(at) {
			latest, at = strings.TrimSpace(event.Reason+": "+event.Message), event.At
		}
	}
	if latest == "" {
		return strings.TrimSpace(status.Reason)
	}
	return trimReason(latest)
}

// trimReason keeps a health detail readable. Kubernetes messages carry volume
// identifiers and RPC wrappers that push the one informative clause off the
// end of any screen showing several of them.
func trimReason(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	const limit = 160
	if len(message) <= limit {
		return message
	}
	return message[:limit] + "…"
}
