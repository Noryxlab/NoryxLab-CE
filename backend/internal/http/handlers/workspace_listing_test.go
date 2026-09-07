package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A project's screen asks for one project's workspaces. It used to receive
// every workspace the caller could see anywhere, because this endpoint was the
// only one of its family that ignored the filter the interface sends: opening
// a brand-new project showed the workspaces of another one.
func TestAProjectListsOnlyItsOwnWorkspaces(t *testing.T) {
	projects := memory.NewProjectStore()
	first := project.NewOwned("stef", "meteo", "")
	second := project.NewOwned("stef", "stephane-tests", "")
	for _, item := range []project.Project{first, second} {
		if err := projects.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(first.ID, "stef", access.RoleAdmin)
	accessStore.SetRole(second.ID, "stef", access.RoleAdmin)

	workspaces := memory.NewWorkspaceStore()
	older := workspace.New("vscode", first.ID, "meteo-workspace", "image", "pod-a", "10.0.0.1", "1", "1Gi", "", "")
	newer := workspace.New("vscode", second.ID, "tests-workspace", "image", "pod-b", "10.0.0.2", "1", "1Gi", "", "")
	for _, item := range []workspace.Workspace{older, newer} {
		if err := workspaces.Create(item); err != nil {
			t.Fatal(err)
		}
	}

	h := Handlers{
		projectStore:       projects,
		accessStore:        accessStore,
		workspaceStore:     workspaces,
		workspaceNamespace: "noryx-loads",
		authMode:           "header",
	}

	names := listWorkspaceNames(t, h, "?projectId="+second.ID)
	if len(names) != 1 || names[0] != "tests-workspace" {
		t.Errorf("a project must list only its own workspaces, got %v", names)
	}

	// No filter still means "everything I may see": the global screen relies
	// on it, so narrowing the endpoint outright would have broken that.
	all := listWorkspaceNames(t, h, "")
	if len(all) != 2 {
		t.Errorf("without a project filter the caller sees all their workspaces, got %v", all)
	}
}

func listWorkspaceNames(t *testing.T, h Handlers, query string) []string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces"+query, nil)
	request.Header.Set("X-Noryx-User", "stef")
	recorder := httptest.NewRecorder()
	h.ListWorkspaces(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		names = append(names, item.Name)
	}
	return names
}
