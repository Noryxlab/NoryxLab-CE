package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

const testMasterKey = "0123456789abcdef0123456789abcdef"

func projectVariableFixture(t *testing.T) (Handlers, project.Project) {
	t.Helper()
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "meteo", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "editor-user", access.RoleEditor)
	accessStore.SetRole(item.ID, "viewer-user", access.RoleViewer)

	return Handlers{
		projectStore:         projects,
		accessStore:          accessStore,
		projectVariableStore: memory.NewProjectVariableStore(),
		secretStore:          memory.NewSecretStore(),
		secretsMasterKey:     testMasterKey,
		authMode:             "header",
	}, item
}

func setVariable(t *testing.T, h Handlers, projectID, user, name, value string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"value":"` + value + `","description":"where the tracking server lives"}`)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/variables/"+name, body)
	request.SetPathValue("projectID", projectID)
	request.SetPathValue("name", name)
	request.Header.Set("X-Noryx-User", user)
	recorder := httptest.NewRecorder()
	h.UpsertProjectVariable(recorder, request)
	return recorder
}

func listVariables(t *testing.T, h Handlers, projectID, user string) (items []projectVariableView, canRead bool) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/variables", nil)
	request.SetPathValue("projectID", projectID)
	request.Header.Set("X-Noryx-User", user)
	recorder := httptest.NewRecorder()
	h.ListProjectVariables(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items         []projectVariableView `json:"items"`
		CanReadValues bool                  `json:"canReadValues"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Items, payload.CanReadValues
}

// The point of the feature: a colleague on the project gets the same value,
// without recreating anything by hand.
func TestAVariableSetByOneMemberIsReadByAnother(t *testing.T) {
	h, item := projectVariableFixture(t)
	if recorder := setVariable(t, h, item.ID, "editor-user", "MLFLOW_TRACKING_URI", "https://mlflow.example.org"); recorder.Code != http.StatusOK {
		t.Fatalf("setting the variable: %d %s", recorder.Code, recorder.Body.String())
	}

	items, canRead := listVariables(t, h, item.ID, "owner")
	if !canRead || len(items) != 1 || items[0].Value != "https://mlflow.example.org" {
		t.Fatalf("the owner should read the value, got %+v (canRead=%v)", items, canRead)
	}
	if items[0].UpdatedBy != "editor-user" {
		t.Errorf("the screen should say who set it, got %q", items[0].UpdatedBy)
	}
}

// A viewer sees what a workload expects without seeing what it is worth. They
// cannot launch either, so the value would never reach their shell.
func TestAViewerSeesTheNamesAndNotTheValues(t *testing.T) {
	h, item := projectVariableFixture(t)
	setVariable(t, h, item.ID, "editor-user", "BUCKET", "s3://private")

	items, canRead := listVariables(t, h, item.ID, "viewer-user")
	if canRead {
		t.Error("a viewer must not be told they can read values")
	}
	if len(items) != 1 || items[0].Name != "BUCKET" {
		t.Fatalf("a viewer should still see the names, got %+v", items)
	}
	if items[0].Value != "" {
		t.Errorf("a viewer read a value: %q", items[0].Value)
	}

	if recorder := setVariable(t, h, item.ID, "viewer-user", "BUCKET", "s3://mine"); recorder.Code != http.StatusForbidden {
		t.Errorf("a viewer must not set a variable, got %d", recorder.Code)
	}
}

func TestAStrangerSeesNothing(t *testing.T) {
	h, item := projectVariableFixture(t)
	setVariable(t, h, item.ID, "editor-user", "BUCKET", "s3://private")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+item.ID+"/variables", nil)
	request.SetPathValue("projectID", item.ID)
	request.Header.Set("X-Noryx-User", "stranger")
	recorder := httptest.NewRecorder()
	h.ListProjectVariables(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("a stranger read a project's variables: %d %s", recorder.Code, recorder.Body.String())
	}
}

// The value is what the tool expects to find, under the name the tool looks
// for, beside the launching person's own secrets.
func TestAWorkloadReceivesTheProjectVariablesUnderTheirOwnNames(t *testing.T) {
	h, item := projectVariableFixture(t)
	setVariable(t, h, item.ID, "editor-user", "MLFLOW_TRACKING_URI", "https://mlflow.example.org")

	data, err := h.workloadEnvData(item.ID, "editor-user")
	if err != nil {
		t.Fatal(err)
	}
	if data["MLFLOW_TRACKING_URI"] != "https://mlflow.example.org" {
		t.Errorf("the workload did not receive the variable: %+v", data)
	}
}

// Values are encrypted at rest. The screen says these are not for credentials;
// people put connection strings in them anyway, and a database dump should not
// be where they find out it mattered.
func TestTheStoredValueIsNotTheValue(t *testing.T) {
	h, item := projectVariableFixture(t)
	setVariable(t, h, item.ID, "editor-user", "DSN", "postgres://user:hunter2@db/app")

	stored, found, err := h.projectVariableStore.GetByName(item.ID, "DSN")
	if err != nil || !found {
		t.Fatal("the variable was not stored")
	}
	if strings.Contains(stored.ValueEncrypted, "hunter2") {
		t.Error("the value is stored in clear")
	}
}

func TestAVariableCannotTakeAPlatformName(t *testing.T) {
	h, item := projectVariableFixture(t)
	for _, name := range []string{"NORYX_SECRET_X", "PATH", "with-dash"} {
		if recorder := setVariable(t, h, item.ID, "editor-user", name, "x"); recorder.Code != http.StatusBadRequest {
			t.Errorf("%s was accepted: %d", name, recorder.Code)
		}
	}
}
