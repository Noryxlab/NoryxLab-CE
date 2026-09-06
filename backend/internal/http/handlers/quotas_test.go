package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/quota"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A quota is only worth having if it refuses, and only bearable if the refusal
// says what to do about it.

func TestAQuotaRefusesTheLaunchThatWouldExceedItAndNamesTheLimit(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.quotaStore = memory.NewQuotaStore()
	if err := h.quotaStore.Set(quota.Quota{ProjectID: item.ID, MaxVCPU: 2}); err != nil {
		t.Fatal(err)
	}

	// One 2x8 workspace already running fills the quota exactly.
	running := workspace.New("jupyter", item.ID, "w1", "image", "pod", "svc", "2", "8Gi", "", "token")
	running.Status = "running"
	if err := h.workspaceStore.Create(running); err != nil {
		t.Fatal(err)
	}

	recorder := launch(t, h, item.ID, "1x4", "")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var answer map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer["code"] != "project_quota_reached" {
		t.Errorf("the refusal must be identifiable by a client, got %v", answer["code"])
	}
	message, _ := answer["error"].(string)
	for _, expected := range []string{"vCPU", "in use", "limit"} {
		if !strings.Contains(message, expected) {
			t.Errorf("the refusal must say %q; got %q", expected, message)
		}
	}
	// The usage travels with the refusal: the person reading it should not
	// have to make a second call to find out what is running.
	if _, ok := answer["usage"]; !ok {
		t.Error("the refusal must carry the current usage")
	}
}

func TestALaunchWithinTheQuotaProceeds(t *testing.T) {
	h, runner, item := launchFixture(t, "")
	h.quotaStore = memory.NewQuotaStore()
	if err := h.quotaStore.Set(quota.Quota{ProjectID: item.ID, MaxVCPU: 8}); err != nil {
		t.Fatal(err)
	}

	if recorder := launch(t, h, item.ID, "1x4", ""); recorder.Code >= 300 {
		t.Fatalf("a launch within the quota must proceed, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.pods) != 1 {
		t.Fatalf("expected the pod to be created, got %d", len(runner.pods))
	}
}

// A project with no quota behaves exactly as it did before quotas existed.
// That is the only migration that cannot break a running platform.
func TestAProjectWithoutAQuotaIsUnlimited(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.quotaStore = memory.NewQuotaStore()

	for i := 0; i < 3; i++ {
		if recorder := launch(t, h, item.ID, "4x16", ""); recorder.Code >= 300 {
			t.Fatalf("launch %d was refused: %d %s", i, recorder.Code, recorder.Body.String())
		}
	}
}

// What is not running does not count. A workspace that failed last week is not
// occupying a node, and counting it would lock a project out of its own quota.
func TestOnlyRunningWorkloadsCountTowardsTheQuota(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.quotaStore = memory.NewQuotaStore()
	if err := h.quotaStore.Set(quota.Quota{ProjectID: item.ID, MaxWorkspaces: 1}); err != nil {
		t.Fatal(err)
	}

	for _, state := range []string{"failed", "stopped"} {
		gone := workspace.New("jupyter", item.ID, "old-"+state, "image", "pod", "svc", "1", "4Gi", "", "token")
		gone.Status = state
		if err := h.workspaceStore.Create(gone); err != nil {
			t.Fatal(err)
		}
	}

	if recorder := launch(t, h, item.ID, "1x4", ""); recorder.Code >= 300 {
		t.Fatalf("finished workloads must not fill a quota, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// A quota belongs to one project.
func TestAQuotaDoesNotReachAnotherProject(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.quotaStore = memory.NewQuotaStore()
	if err := h.quotaStore.Set(quota.Quota{ProjectID: "some-other-project", MaxVCPU: 1}); err != nil {
		t.Fatal(err)
	}
	if recorder := launch(t, h, item.ID, "4x16", ""); recorder.Code >= 300 {
		t.Fatalf("another project's quota must not apply here, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// An empty quota is a removal, not a limit of zero. Storing zeroes would be a
// project that can run nothing, which is not what "no quota" means.
func TestAnEmptyQuotaRemovesTheLimitRatherThanForbiddingEverything(t *testing.T) {
	store := memory.NewQuotaStore()
	if err := store.Set(quota.Quota{ProjectID: "p1", MaxVCPU: 4}); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(quota.Quota{ProjectID: "p1"}); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get("p1"); found {
		t.Error("an empty quota must remove the row, not store a limit of zero")
	}
}

// The reading is open to members: somebody refused a launch has to see why
// without opening a ticket.
func TestAMemberCanReadTheQuotaAndTheUsage(t *testing.T) {
	h, _, item := launchFixture(t, "")
	h.quotaStore = memory.NewQuotaStore()
	if err := h.quotaStore.Set(quota.Quota{ProjectID: item.ID, MaxVCPU: 4, MaxWorkspaces: 2}); err != nil {
		t.Fatal(err)
	}
	running := workspace.New("jupyter", item.ID, "w1", "image", "pod", "svc", "1", "4Gi", "", "token")
	running.Status = "running"
	if err := h.workspaceStore.Create(running); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+item.ID+"/quota", nil)
	request.SetPathValue("projectID", item.ID)
	request.Header.Set("X-Noryx-User", "member")
	recorder := httptest.NewRecorder()
	h.GetProjectQuota(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("a member must be able to read the quota, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var answer struct {
		Quota quota.Quota `json:"quota"`
		Usage quota.Usage `json:"usage"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Quota.MaxVCPU != 4 || answer.Usage.Workspaces != 1 || answer.Usage.VCPU != 1 {
		t.Fatalf("the answer must carry both the limit and what is running: %+v", answer)
	}
}
