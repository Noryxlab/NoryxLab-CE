package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A pod that is pending, and Kubernetes' own explanation for it.
//
// The embedded interface is left nil deliberately: anything this check calls
// beyond these two methods panics loudly instead of returning a silent zero.
type pendingRuntime struct {
	noryxruntime.Runner
	phase  string
	events []noryxruntime.PodEvent
}

func (r pendingRuntime) GetPodStatus(string) (noryxruntime.PodStatus, error) {
	return noryxruntime.PodStatus{Phase: r.phase}, nil
}
func (r pendingRuntime) GetPodLogs(string, int) (string, error) { return "", nil }
func (r pendingRuntime) RestartPod(string) error                { return nil }
func (r pendingRuntime) GetPodEvents(string) ([]noryxruntime.PodEvent, error) {
	return r.events, nil
}

func handlersWithWorkspace(runtime noryxruntime.Runner, age time.Duration) Handlers {
	store := memory.NewWorkspaceStore()
	_ = store.Create(workspace.Workspace{
		ID: "w1", Name: "vscode-samy", PodName: "wks-7aa2799e4a78",
		CreatedAt: time.Now().UTC().Add(-age),
	})
	return Handlers{workspaceStore: store, runtime: runtime}
}

// The condition that cost two people a morning: the pod was pending for four
// hours, Kubernetes recorded why twice a minute, and nothing asked.
func TestAWorkspaceThatNeverStartsIsReported(t *testing.T) {
	runtime := pendingRuntime{phase: "Pending", events: []noryxruntime.PodEvent{{
		Type: "Warning", Reason: "FailedAttachVolume", At: time.Now().UTC(),
		Message: "AttachVolume.Attach failed for volume \"pvc-45fd3bd1\" : rpc error: code = Aborted desc = volume is not ready for workloads",
	}}}
	alerts := handlersWithWorkspace(runtime, 4*time.Hour).stuckWorkspaceAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected one alert, got %d", len(alerts))
	}
	// Four hours is not slow, it is stuck, and it will not resolve itself.
	if alerts[0].Severity != healthCritical {
		t.Errorf("a four-hour wait should be critical, got %v", alerts[0].Severity)
	}
	// The Kubernetes wording survives: an operator searches for the exact
	// string, and a friendlier paraphrase matches nothing.
	if !strings.Contains(alerts[0].Detail, "FailedAttachVolume") {
		t.Errorf("the reason was lost: %s", alerts[0].Detail)
	}
	if !strings.Contains(alerts[0].Detail, "vscode-samy") {
		t.Errorf("the workspace was not named: %s", alerts[0].Detail)
	}
}

func TestAWorkspaceStillStartingIsNotAnIncident(t *testing.T) {
	// A large image on a cold node takes minutes. Reporting that would teach
	// an operator to ignore this alert, which is worse than not having it.
	runtime := pendingRuntime{phase: "Pending"}
	if alerts := handlersWithWorkspace(runtime, 2*time.Minute).stuckWorkspaceAlerts(); len(alerts) != 0 {
		t.Fatalf("a two-minute-old workspace was reported: %v", alerts)
	}
	// Past the grace period it is worth saying, without being critical yet.
	alerts := handlersWithWorkspace(runtime, 20*time.Minute).stuckWorkspaceAlerts()
	if len(alerts) != 1 || alerts[0].Severity != healthWarning {
		t.Fatalf("expected one warning at twenty minutes, got %v", alerts)
	}
}

func TestARunningWorkspaceIsNotReported(t *testing.T) {
	runtime := pendingRuntime{phase: "Running"}
	if alerts := handlersWithWorkspace(runtime, 30*time.Hour).stuckWorkspaceAlerts(); len(alerts) != 0 {
		t.Fatalf("a running workspace was reported as stuck: %v", alerts)
	}
}
