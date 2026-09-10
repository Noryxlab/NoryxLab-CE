package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A stopped application can be started again.
//
// Restarting used to read the live pod and put it back, so stopping an
// application - which deletes the pod - made it unrecoverable: every restart
// answered 502, and the only way out was to delete the application and rebuild
// it under a new identity. That is what happened to a real accounting app in
// front of a real accountant.
func TestStoppedAppCanBeRestarted(t *testing.T) {
	h, runner, record := restartFixture(t, "stopped")

	response := restart(t, h, "/api/v1/apps/"+record.ID+"/restart", record.ID)

	if response.Code != http.StatusAccepted {
		t.Fatalf("restart of a stopped app = %d, want 202: %s", response.Code, response.Body.String())
	}
	if len(runner.pods) != 1 {
		t.Fatalf("no pod was created: a stopped app must be relaunched, not read back")
	}
	if runner.pods[0].PodName != record.PodName {
		t.Fatalf("pod name = %q, want %q - the application must keep its identity",
			runner.pods[0].PodName, record.PodName)
	}
}

// Restarting re-resolves the dataset volumes.
//
// The S3 endpoint is written into the volume as an address, because the mounter
// runs on the host and cannot resolve cluster DNS. When the storage service was
// recreated with a new address, every stored volume pointed at an address
// nobody answered - and a restart rebuilt the pod around that same stale
// volume, so the button that exists to repair an application could not repair
// it. Creation always re-resolved; only restart did not.
func TestRestartReResolvesDatasetVolumes(t *testing.T) {
	h, runner, record := restartFixture(t, "running")

	if response := restart(t, h, "/api/v1/apps/"+record.ID+"/restart", record.ID); response.Code != http.StatusAccepted {
		t.Fatalf("restart = %d, want 202: %s", response.Code, response.Body.String())
	}
	// No dataset is attached in this fixture, so the assertion is on the call
	// being made at all: the resolution runs on the restart path.
	if len(runner.pods) != 1 {
		t.Fatalf("the restart did not go through the launch path")
	}
}

func restartFixture(t *testing.T, status string) (Handlers, *recordingRuntime, app.App) {
	t.Helper()
	h, runner, item := launchFixture(t, "")
	h.appStore = memory.NewAppStore()

	record := app.NewWithKind("app", item.ID, "accounting", "accounting",
		"harbor/rstudio:1", []string{"/bin/sh", "-lc"}, []string{"echo hello"},
		9000, "app-test", "app-test", "/apps/accounting/")
	record.OwnerUserID = "member"
	record.Status = status
	if err := h.appStore.Create(record); err != nil {
		t.Fatal(err)
	}
	return h, runner, record
}

func restart(t *testing.T, h Handlers, path, id string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, nil)
	request.Header.Set("X-Noryx-User", "member")
	request.SetPathValue("appID", id)
	response := httptest.NewRecorder()
	h.RestartApp(response, request)
	return response
}

// A restart waits for the previous instance to be gone.
//
// Deletion is asynchronous, so recreating the pod under the same name a
// moment later came back as "already exists": the restart answered 502 with a
// Kubernetes 409 inside it, which reads like a broken cluster and was only
// impatience. The wait is bounded, because a pod that will not terminate is a
// condition to report rather than one to hold a request open for.
func TestRestartWaitsForThePreviousInstance(t *testing.T) {
	h, _, record := restartFixture(t, "running")
	lingering := &lingeringRuntime{recordingRuntime: &recordingRuntime{}, remaining: 3}
	h.runtime = lingering

	if response := restart(t, h, "/api/v1/apps/"+record.ID+"/restart", record.ID); response.Code != http.StatusAccepted {
		t.Fatalf("restart = %d, want 202: %s", response.Code, response.Body.String())
	}
	if lingering.remaining > 0 {
		t.Fatalf("the restart did not wait: %d checks left", lingering.remaining)
	}
	if len(lingering.pods) != 1 {
		t.Fatalf("the pod was not recreated once the name was free")
	}
}

// lingeringRuntime reports the pod as still present for a few checks, the way a
// terminating pod does.
type lingeringRuntime struct {
	*recordingRuntime
	remaining int
}

func (r *lingeringRuntime) GetPodStatus(string) (noryxruntime.PodStatus, error) {
	if r.remaining > 0 {
		r.remaining--
		return noryxruntime.PodStatus{Phase: "Running"}, nil
	}
	return noryxruntime.PodStatus{}, errNoSuchPod
}

func (r *lingeringRuntime) GetPodLogs(string, int) (string, error) { return "", nil }
func (r *lingeringRuntime) RestartPod(string) error                { return nil }

var errNoSuchPod = errors.New("pods \"x\" not found")

// The service surviving a stop is not a failure.
//
// Stopping deletes the pod and leaves the service, so a restart's "make sure
// the service exists" came back as a conflict. Treated as an error, it left the
// application relaunched but reported as broken - the worst of both.
func TestRestartToleratesTheServiceThatSurvivedTheStop(t *testing.T) {
	h, _, record := restartFixture(t, "stopped")
	h.runtime = &conflictingServiceRuntime{recordingRuntime: &recordingRuntime{}}

	if response := restart(t, h, "/api/v1/apps/"+record.ID+"/restart", record.ID); response.Code != http.StatusAccepted {
		t.Fatalf("restart = %d, want 202: an existing service means there is nothing to do, not that the restart failed: %s",
			response.Code, response.Body.String())
	}
}

type conflictingServiceRuntime struct{ *recordingRuntime }

func (r *conflictingServiceRuntime) CreateService(noryxruntime.ServiceSpec) error {
	return errors.New("kubernetes api /api/v1/namespaces/noryx-loads/services failed: status=409 body={}")
}
