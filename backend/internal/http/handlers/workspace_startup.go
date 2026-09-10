package handlers

import (
	"net/http"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Why a workspace is not starting, in words somebody can act on.
//
// When a workspace hangs, the platform showed "launching" and nothing else. The
// answer existed - Kubernetes had recorded that a volume could not be attached,
// or that an image could not be pulled - but it lived in `kubectl describe`,
// which is not a place a researcher goes. Waiting was the only strategy, and it
// worked or it did not.
//
// This is deliberately not a translation of Kubernetes into French. It is five
// steps, in the order they happen, each either done, in progress, failed or not
// yet reached. The engine's own words are kept alongside for whoever wants
// them, but nobody has to read them to know where it stopped.

type startupStep struct {
	// Key names the step for the interface, which supplies the wording.
	Key string `json:"key"`
	// State is one of: done, running, failed, waiting.
	State string `json:"state"`
	// Detail is the platform's own explanation when something went wrong,
	// written for a reader rather than an operator.
	Detail string `json:"detail,omitempty"`
	// Technical is what Kubernetes said, kept for whoever wants it and shown
	// behind a disclosure rather than in the reader's face.
	Technical string `json:"technical,omitempty"`
}

type startupReport struct {
	Steps []startupStep `json:"steps"`
	// Stuck says the workspace is not progressing on its own, so the interface
	// can stop implying that waiting longer will help.
	Stuck bool `json:"stuck"`
}

func (h Handlers) GetWorkspaceStartup(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	record, found, err := h.workspaceStore.GetByID(strings.TrimSpace(r.PathValue("workspaceID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workspace not found"})
		return
	}
	if !h.requireProjectMember(w, record.ProjectID, identity.UserID(), "workspace startup") {
		return
	}

	var events []noryxruntime.PodEvent
	if reader, ok := h.runtime.(noryxruntime.PodEventReader); ok && strings.TrimSpace(record.PodName) != "" {
		events, _ = reader.GetPodEvents(record.PodName)
	}
	var status noryxruntime.PodStatus
	if operator, ok := h.runtime.(noryxruntime.PodOperator); ok && strings.TrimSpace(record.PodName) != "" {
		status, _ = operator.GetPodStatus(record.PodName)
	}
	writeJSON(w, http.StatusOK, buildStartupReport(record.Status, status, events))
}

// buildStartupReport turns a pod's phase and its events into the five steps.
// Kept free of the request so it can be tested on the situations that actually
// happen, which is how the volume case below was written.
// mentionsVolume reports whether a scheduling failure is really about storage.
//
// Matched on the vocabulary Kubernetes uses rather than on an exact sentence:
// the wording has changed between versions, and a diagnosis that breaks on an
// upgrade is a diagnosis nobody can rely on.
func mentionsVolume(message string) bool {
	lowered := strings.ToLower(message)
	for _, marker := range []string{
		"persistentvolumeclaim",
		"unbound immediate",
		"volume node affinity",
		"had volume node affinity conflict",
		"volume attachment",
	} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

func buildStartupReport(recorded string, status noryxruntime.PodStatus, events []noryxruntime.PodEvent) startupReport {
	report := startupReport{Steps: []startupStep{
		{Key: "requested", State: "done"},
		{Key: "scheduled", State: "waiting"},
		{Key: "image", State: "waiting"},
		{Key: "storage", State: "waiting"},
		{Key: "environment", State: "waiting"},
	}}
	at := func(key string) *startupStep {
		for index := range report.Steps {
			if report.Steps[index].Key == key {
				return &report.Steps[index]
			}
		}
		return nil
	}

	for _, event := range events {
		switch event.Reason {
		case "Scheduled":
			at("scheduled").State = "done"
		case "FailedScheduling":
			// Kubernetes reports a pod waiting on a volume as unschedulable,
			// which is true and useless: the reader is told the cluster is
			// full when a dataset volume is the thing that failed. It says so
			// in the message - "unbound immediate PersistentVolumeClaims" -
			// so the message is what decides which step failed.
			//
			// This mattered more than a wording detail. A researcher whose
			// dataset could not be provisioned was told to wait for capacity
			// that was already there, and a confidently wrong cause is worse
			// than none: it sends everybody, humans and any assistant reading
			// this report, after the wrong thing.
			if mentionsVolume(event.Message) {
				at("storage").State = "failed"
				at("storage").Detail = "storage_unavailable"
				at("storage").Technical = event.Message
			} else {
				at("scheduled").State = "failed"
				at("scheduled").Detail = "no_capacity"
				at("scheduled").Technical = event.Message
			}
			report.Stuck = true
		case "ProvisioningFailed":
			// The volume was never created, so there is nothing to attach and
			// FailedMount will never come.
			at("storage").State = "failed"
			at("storage").Detail = "storage_unavailable"
			at("storage").Technical = event.Message
			report.Stuck = true
		case "Pulling":
			at("scheduled").State = "done"
			at("image").State = "running"
		case "Pulled":
			at("scheduled").State = "done"
			at("image").State = "done"
		case "Failed", "ErrImagePull", "BackOff":
			if strings.Contains(strings.ToLower(event.Message), "image") {
				at("image").State = "failed"
				at("image").Detail = "image_unavailable"
				at("image").Technical = event.Message
				report.Stuck = true
			}
		case "SuccessfulAttachVolume":
			at("storage").State = "done"
		case "FailedAttachVolume", "FailedMount":
			// The case that cost an afternoon on EMSE: the S3 driver was not
			// installed, so every workspace with a dataset waited for an
			// attachment that would never happen, showing nothing at all.
			at("storage").State = "failed"
			at("storage").Detail = "storage_unavailable"
			at("storage").Technical = event.Message
			report.Stuck = true
		case "Created", "Started":
			at("image").State = "done"
			at("storage").State = "done"
			at("environment").State = "running"
		}
	}

	switch strings.ToLower(strings.TrimSpace(status.Phase)) {
	case "running":
		for _, key := range []string{"scheduled", "image", "storage"} {
			if at(key).State != "failed" {
				at(key).State = "done"
			}
		}
		if strings.EqualFold(recorded, "running") {
			at("environment").State = "done"
		} else if at("environment").State == "waiting" {
			at("environment").State = "running"
		}
	case "failed":
		at("environment").State = "failed"
		at("environment").Detail = "environment_failed"
		at("environment").Technical = strings.TrimSpace(status.Reason + " " + status.Message)
		report.Stuck = true
	}
	if status.RestartCount > 2 {
		at("environment").State = "failed"
		at("environment").Detail = "environment_restarting"
		report.Stuck = true
	}
	return report
}
