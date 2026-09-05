package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// recordingRuntime accepts everything and remembers what it was asked for, so
// a test can assert on the pod that *would* be created without a cluster. The
// interesting decisions - the volume size, the tier's limits, the namespace -
// are all made before Kubernetes is ever called.
type recordingRuntime struct {
	mu     sync.Mutex
	pods   []noryxruntime.PodSpec
	claims []noryxruntime.PersistentVolumeClaimSpec
	jobs   []noryxruntime.JobSpec
}

func (r *recordingRuntime) CreatePod(spec noryxruntime.PodSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pods = append(r.pods, spec)
	return nil
}

func (r *recordingRuntime) CreatePersistentVolumeClaim(spec noryxruntime.PersistentVolumeClaimSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claims = append(r.claims, spec)
	return nil
}

func (r *recordingRuntime) DeletePersistentVolumeClaim(string) error       { return nil }
func (r *recordingRuntime) EnsureS3Volume(noryxruntime.S3VolumeSpec) error { return nil }
func (r *recordingRuntime) DeleteS3Volume(string) error                    { return nil }
func (r *recordingRuntime) DeletePod(string) error                         { return nil }
func (r *recordingRuntime) CreateService(noryxruntime.ServiceSpec) error   { return nil }
func (r *recordingRuntime) DeleteService(string) error                     { return nil }
func (r *recordingRuntime) CreateBuild(noryxruntime.BuildSpec) error       { return nil }
func (r *recordingRuntime) CreateJob(spec noryxruntime.JobSpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, spec)
	return nil
}
func (r *recordingRuntime) DeleteJob(string) error                       { return nil }
func (r *recordingRuntime) CreateCronJob(noryxruntime.CronJobSpec) error { return nil }
func (r *recordingRuntime) DeleteCronJob(string) error                   { return nil }
func (r *recordingRuntime) CreateSecret(noryxruntime.SecretSpec) error   { return nil }
func (r *recordingRuntime) DeleteSecret(string) error                    { return nil }

func launchFixture(t *testing.T, storageSize string) (Handlers, *recordingRuntime, project.Project) {
	t.Helper()
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Launch project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	if storageSize != "" {
		if err := projects.UpdateWorkspaceStorageSize(item.ID, storageSize); err != nil {
			t.Fatal(err)
		}
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "member", access.RoleEditor)

	runner := &recordingRuntime{}
	h := Handlers{
		projectStore:         projects,
		accessStore:          accessStore,
		workspaceStore:       memory.NewWorkspaceStore(),
		sessionStore:         memory.NewSessionStore(),
		hardwareTierStore:    memory.NewHardwareTierStore(),
		buildStore:           memory.NewBuildStore(),
		projectResourceStore: memory.NewProjectResourceStore(),
		repositoryStore:      memory.NewRepositoryStore(),
		datasetStore:         memory.NewDatasetStore(),
		datasourceStore:      memory.NewDatasourceStore(),
		secretStore:          memory.NewSecretStore(),
		podStore:             memory.NewPodStore(),
		auditStore:           memory.NewAuditStore(),
		// The platform's own image, so the environment check passes without a
		// build: what is under test here is the launch, not the catalogue.
		workspaceJupyterImage: "harbor/jupyter:1",
		runtime:               runner,
		workspaceNamespace:    "noryx-loads",
		workspacePVCEnabled:   true,
		workspacePVCSize:      "10Gi",
		workspacePVCClass:     "longhorn",
		authMode:              "header",
	}
	return h, runner, item
}

func launch(t *testing.T, h Handlers, projectID, tier, storage string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(createWorkspaceRequest{
		ProjectID: projectID, IDE: "jupyter", Image: "harbor/jupyter:1",
		HardwareTier: tier, StorageSize: storage,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
	request.Header.Set("X-Noryx-User", "member")
	recorder := httptest.NewRecorder()
	h.CreateWorkspace(recorder, request)
	return recorder
}

// The capacity comes from the project. It moved off the launch form on
// 2026-09-05, and a setting that is stored but never read would look identical
// on screen while every workspace kept the old size.
func TestALaunchTakesItsVolumeFromTheProject(t *testing.T) {
	h, runner, item := launchFixture(t, "50Gi")

	recorder := launch(t, h, item.ID, "", "")
	if recorder.Code >= 300 {
		t.Fatalf("the launch failed: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.claims) != 1 {
		t.Fatalf("expected one volume claim, got %d", len(runner.claims))
	}
	if runner.claims[0].Size != "50Gi" {
		t.Errorf("the volume must follow the project setting, got %q", runner.claims[0].Size)
	}
}

func TestAProjectWithNoSettingFollowsThePlatformDefault(t *testing.T) {
	h, runner, item := launchFixture(t, "")

	if recorder := launch(t, h, item.ID, "", ""); recorder.Code >= 300 {
		t.Fatalf("the launch failed: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.claims) != 1 || runner.claims[0].Size != "10Gi" {
		t.Fatalf("expected the platform default of 10Gi, got %+v", runner.claims)
	}
}

// A caller still sending storageSize is answered, not ignored. A field that
// silently does nothing leaves its author believing it works.
func TestAStorageSizeSentByAClientIsRefusedRatherThanIgnored(t *testing.T) {
	h, _, item := launchFixture(t, "50Gi")

	recorder := launch(t, h, item.ID, "", "100Gi")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "project setting") {
		t.Errorf("the error must say where the size now lives, got %s", recorder.Body.String())
	}
}

// The tier decides the machine, and an unknown one is refused rather than
// quietly replaced by the default - a pipeline asking for a large machine and
// silently getting a small one is a failure that looks like slowness.
func TestAnUnknownTierIsRefused(t *testing.T) {
	h, _, item := launchFixture(t, "")

	recorder := launch(t, h, item.ID, "enormous", "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown tier, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestTheDefaultTierSizesThePod(t *testing.T) {
	h, runner, item := launchFixture(t, "")

	if recorder := launch(t, h, item.ID, "2x8", ""); recorder.Code >= 300 {
		t.Fatalf("the launch failed: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.pods) != 1 {
		t.Fatalf("expected one pod, got %d", len(runner.pods))
	}
	pod := runner.pods[0]
	if pod.CPULimit != "2" || pod.MemLimit != "8Gi" {
		t.Errorf("the pod must carry the tier's limits, got cpu=%q memory=%q", pod.CPULimit, pod.MemLimit)
	}
	// The isolation label is added a layer down, by the Kubernetes runtime, so
	// it is asserted there - see TestEveryUserWorkloadCarriesTheIsolationLabel.
	// What this layer decides is which project and which tier the pod belongs
	// to, and both end up on labels an operator greps for during an incident.
	if pod.Labels["noryx.io/project-id"] != item.ID {
		t.Errorf("a workspace must be traceable to its project, got %v", pod.Labels)
	}
	if pod.Labels["noryx.io/hardware-tier"] != "2x8" {
		t.Errorf("a workspace must record the tier it was given, got %v", pod.Labels)
	}
}

// Jobs run on the same rails as workspaces, so the same three questions apply:
// who may start one, on which machine, and what happens when the answer is no.
func runJob(t *testing.T, h Handlers, projectID, tier, user string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(createJobRequest{
		ProjectID: projectID, Name: "nightly", Image: "harbor/jupyter:1", HardwareTier: tier,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
	request.Header.Set("X-Noryx-User", user)
	recorder := httptest.NewRecorder()
	h.CreateJob(recorder, request)
	return recorder
}

func TestAJobRunsOnTheTierItAsksFor(t *testing.T) {
	h, runner, item := launchFixture(t, "")
	h.jobStore = memory.NewJobStore()

	recorder := runJob(t, h, item.ID, "2x8", "member")
	if recorder.Code >= 300 {
		t.Fatalf("the job was refused: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(runner.jobs))
	}
	if runner.jobs[0].CPULimit != "2" || runner.jobs[0].MemLimit != "8Gi" {
		t.Errorf("the job must carry the tier's limits, got cpu=%q memory=%q", runner.jobs[0].CPULimit, runner.jobs[0].MemLimit)
	}
}

func TestAnUnknownTierIsRefusedForAJobToo(t *testing.T) {
	h, runner, item := launchFixture(t, "")
	h.jobStore = memory.NewJobStore()

	recorder := runJob(t, h, item.ID, "enormous", "member")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.jobs) != 0 {
		t.Error("nothing must reach the cluster when the request is refused")
	}
}

// A viewer may read a project and may not spend its compute. The rule is the
// same as for a workspace, and it is worth asserting separately: they are two
// handlers, and only one of them was ever tested.
func TestAViewerCannotRunAJob(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.jobStore = memory.NewJobStore()
	h.accessStore.SetRole(item.ID, "reader", access.RoleViewer)

	recorder := runJob(t, h, item.ID, "", "reader")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("a viewer must not run a job, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
