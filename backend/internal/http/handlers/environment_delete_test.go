package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// An environment's identifier carries its image reference, so it holds
// slashes. The route took a single path segment, so deleting an environment
// never reached this handler at all: the router answered "Method Not Allowed",
// which is what the person clicking Delete was shown.
//
// The route is a wildcard now. This test covers what the handler does once it
// is reached: every revision of the environment goes, and only that
// environment's.
func TestDeletingAnEnvironmentRemovesEveryRevisionOfIt(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("stef", "meteo", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "stef", access.RoleEditor)

	builds := memory.NewBuildStore()
	repository := "harbor.emse.local/noryx-environments/proj-training"
	for _, tag := range []string{":1", ":2"} {
		record := build.New(item.ID, "", "", "Dockerfile", ".", repository+tag, "job"+tag)
		if err := builds.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	other := build.New(item.ID, "", "", "Dockerfile", ".", "harbor.emse.local/noryx-environments/proj-other:1", "job-other")
	if err := builds.Create(other); err != nil {
		t.Fatal(err)
	}

	h := Handlers{
		projectStore: projects,
		accessStore:  accessStore,
		buildStore:   builds,
		authMode:     "header",
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/x", nil)
	request.SetPathValue("environmentID", item.ID+"|"+repository)
	request.Header.Set("X-Noryx-User", "stef")
	recorder := httptest.NewRecorder()
	h.DeleteEnvironment(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	remaining, err := builds.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].DestinationImage != other.DestinationImage {
		t.Errorf("expected only the other environment to survive, got %d builds", len(remaining))
	}
}

// Deleting something that is not there is a 404 and not a silent success: a
// screen that says "deleted" about a row it did not touch is worse than an
// error.
func TestDeletingAnEnvironmentThatIsNotThereSaysSo(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("stef", "meteo", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "stef", access.RoleEditor)

	h := Handlers{
		projectStore: projects,
		accessStore:  accessStore,
		buildStore:   memory.NewBuildStore(),
		authMode:     "header",
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/x", nil)
	request.SetPathValue("environmentID", item.ID+"|harbor/x/nothing")
	request.Header.Set("X-Noryx-User", "stef")
	recorder := httptest.NewRecorder()
	h.DeleteEnvironment(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status %d, expected 404: %s", recorder.Code, recorder.Body.String())
	}
}
