package handlers

import (
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

func stepState(report startupReport, key string) startupStep {
	for _, step := range report.Steps {
		if step.Key == key {
			return step
		}
	}
	return startupStep{}
}

// The case that cost an afternoon on EMSE. The S3 driver was not installed, so
// every workspace with a dataset waited for an attachment that would never
// happen - and the platform showed "launching" for eight minutes and then
// nothing. The answer was in the pod's events the whole time.
func TestStorageFailureIsNamedRatherThanWaitedOut(t *testing.T) {
	report := buildStartupReport("launching", noryxruntime.PodStatus{Phase: "pending"}, []noryxruntime.PodEvent{
		{Reason: "Scheduled", Message: "assigned to worker-1"},
		{Reason: "FailedAttachVolume", Message: "timed out waiting for external-attacher of ru.yandex.s3.csi"},
	})
	if got := stepState(report, "scheduled"); got.State != "done" {
		t.Fatalf("scheduling should be done, got %q", got.State)
	}
	storage := stepState(report, "storage")
	if storage.State != "failed" || storage.Detail != "storage_unavailable" {
		t.Fatalf("storage step = %+v, want a named failure", storage)
	}
	if storage.Technical == "" {
		t.Fatal("the engine's own words must be kept for whoever wants them")
	}
	if !report.Stuck {
		t.Fatal("a workspace that cannot attach its volume is not going to start by waiting")
	}
}

// A workspace that started reports every step done and nothing stuck, so the
// interface has no reason to show this panel at all.
func TestRunningWorkspaceReportsNothingWrong(t *testing.T) {
	report := buildStartupReport("running", noryxruntime.PodStatus{Phase: "running"}, []noryxruntime.PodEvent{
		{Reason: "Scheduled"}, {Reason: "Pulled"}, {Reason: "SuccessfulAttachVolume"}, {Reason: "Started"},
	})
	for _, step := range report.Steps {
		if step.State != "done" {
			t.Fatalf("step %q = %q, want done", step.Key, step.State)
		}
	}
	if report.Stuck {
		t.Fatal("a running workspace is not stuck")
	}
}

// An image that cannot be pulled stops at the image step, not at storage: the
// point of the report is saying which one, not that something went wrong.
func TestImageFailureStopsAtTheImageStep(t *testing.T) {
	report := buildStartupReport("launching", noryxruntime.PodStatus{Phase: "pending"}, []noryxruntime.PodEvent{
		{Reason: "Scheduled"},
		{Reason: "Failed", Message: "Failed to pull image harbor.lan/x: not found"},
	})
	if stepState(report, "image").Detail != "image_unavailable" {
		t.Fatalf("image step = %+v", stepState(report, "image"))
	}
	if stepState(report, "storage").State != "waiting" {
		t.Fatal("storage was never reached and must not be blamed")
	}
}

// An out-of-memory kill must say so, and say what to do.
//
// Taken from a real pod on the EMSE cluster: phase Failed, status.reason
// empty, and the fact sitting in the container's terminated state. Without
// reading that, the report showed a failed environment with no cause at all -
// a screen that says something broke and nothing else.
func TestAnOutOfMemoryKillExplainsItselfAndTheRemedy(t *testing.T) {
	report := buildStartupReport("running", noryxruntime.PodStatus{
		Phase:       "failed",
		OutOfMemory: true,
	}, nil)

	if !report.OutOfMemory {
		t.Fatal("the report does not carry the out-of-memory flag the interface keys on")
	}
	if !report.Stuck {
		t.Fatal("an out-of-memory kill is not going to resolve itself by waiting")
	}
	var environment startupStep
	for _, step := range report.Steps {
		if step.Key == "environment" {
			environment = step
		}
	}
	if environment.State != "failed" {
		t.Fatalf("environment step = %q, want failed", environment.State)
	}
	if environment.Detail != "environment_out_of_memory" {
		t.Fatalf("detail = %q: the interface cannot tell this from any other failure", environment.Detail)
	}
	if !strings.Contains(environment.Technical, "augmenter le tier") {
		t.Fatalf("the remedy is missing from %q", environment.Technical)
	}
}

// It wins over the diagnoses that would otherwise be written.
//
// A container killed for memory and restarted looks like a restart loop, which
// is true and useless: the loop is the symptom. Reporting that instead sends
// somebody to look at their code.
func TestOutOfMemoryWinsOverARestartLoop(t *testing.T) {
	report := buildStartupReport("running", noryxruntime.PodStatus{
		Phase: "running", RestartCount: 5, OutOfMemory: true,
	}, nil)

	for _, step := range report.Steps {
		if step.Key != "environment" {
			continue
		}
		if step.Detail != "environment_out_of_memory" {
			t.Fatalf("detail = %q, want the memory cause rather than the restart symptom", step.Detail)
		}
	}
}

// And a healthy workload says nothing about memory.
func TestAHealthyWorkspaceCarriesNoMemoryNotice(t *testing.T) {
	report := buildStartupReport("running", noryxruntime.PodStatus{Phase: "running"}, nil)
	if report.OutOfMemory {
		t.Fatal("a running workspace was reported as killed for memory")
	}
}
