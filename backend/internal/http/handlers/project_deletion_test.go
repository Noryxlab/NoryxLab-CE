package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Deleting a project removed its workspaces and left its applications where
// they were. An app is deliberately never reaped, so one whose project has
// gone keeps running against an identifier that resolves to nothing: it costs
// a card, serves an endpoint, and appears on no screen, because every screen
// finds it through the project it no longer has.
func projetAvecCharges(t *testing.T) (Handlers, project.Project) {
	t.Helper()
	projects := memory.NewProjectStore()
	item := project.NewOwned("stef", "projet-a-supprimer", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "stef", access.RoleAdmin)

	workspaces := memory.NewWorkspaceStore()
	if err := workspaces.Create(workspace.New("vscode", item.ID, "un-workspace",
		"image", "pod-a", "10.0.0.1", "1", "1Gi", "", "")); err != nil {
		t.Fatal(err)
	}
	apps := memory.NewAppStore()
	if err := apps.Create(app.New(item.ID, "une-app", "une-app", "image",
		nil, nil, 8080, "pod-b", "svc-b", "")); err != nil {
		t.Fatal(err)
	}

	return Handlers{
		projectStore:   projects,
		accessStore:    accessStore,
		workspaceStore: workspaces,
		appStore:       apps,
		authMode:       "header",
	}, item
}

func projetExiste(t *testing.T, h Handlers, projectID string) bool {
	t.Helper()
	items, err := h.projectStore.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == projectID {
			return true
		}
	}
	return false
}

func supprimer(t *testing.T, h Handlers, projectID, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID+query, nil)
	request.SetPathValue("projectID", projectID)
	request.Header.Set(userHeader, "stef")
	recorder := httptest.NewRecorder()
	h.DeleteProject(recorder, request)
	return recorder
}

// Le refus, et la liste avec lui : on ne decide pas de detruire ce que
// personne ne nous a montre.
func TestUnProjetNonVideEstRefuseAvecSaListe(t *testing.T) {
	h, item := projetAvecCharges(t)

	recorder := supprimer(t, h, item.ID, "")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("code %d, attendu 409 sur un projet qui porte encore des charges", recorder.Code)
	}
	var body struct {
		Code  string `json:"code"`
		Items []struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "project_not_empty" {
		t.Errorf("code = %q", body.Code)
	}
	kinds := map[string]bool{}
	for _, entry := range body.Items {
		kinds[entry.Kind] = true
	}
	if !kinds["workspace"] || !kinds["app"] {
		t.Fatalf("la liste n'annonce pas tout ce qui existe : %+v", body.Items)
	}

	// Et rien n'a ete detruit au passage.
	if !projetExiste(t, h, item.ID) {
		t.Fatal("le projet a ete supprime alors que la suppression etait refusee")
	}
	if items, _ := h.appStore.List(); len(items) != 1 {
		t.Fatalf("%d application(s) apres un refus, attendu 1", len(items))
	}
}

// force=true est l'appelant qui dit avoir lu la liste.
func TestAvecForceLeProjetEtSesChargesPartent(t *testing.T) {
	h, item := projetAvecCharges(t)

	recorder := supprimer(t, h, item.ID, "?force=true")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("code %d, attendu 204 : %s", recorder.Code, recorder.Body.String())
	}
	if projetExiste(t, h, item.ID) {
		t.Error("le projet existe encore")
	}
	if items, _ := h.appStore.List(); len(items) != 0 {
		t.Errorf("%d application(s) survivent au projet qui les portait", len(items))
	}
	if items, _ := h.workspaceStore.List(); len(items) != 0 {
		t.Errorf("%d espace(s) de travail survivent", len(items))
	}
}

// Un projet vide se supprime sans ceremonie : la garde ne doit pas devenir une
// formalite qu'on apprend a cliquer sans lire.
func TestUnProjetVideSeSupprimeSansForce(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("stef", "projet-vide", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "stef", access.RoleAdmin)
	h := Handlers{
		projectStore:   projects,
		accessStore:    accessStore,
		workspaceStore: memory.NewWorkspaceStore(),
		appStore:       memory.NewAppStore(),
		jobStore:       memory.NewJobStore(),
		authMode:       "header",
	}

	if recorder := supprimer(t, h, item.ID, ""); recorder.Code != http.StatusNoContent {
		t.Fatalf("code %d, attendu 204 sur un projet vide : %s", recorder.Code, recorder.Body.String())
	}
}

// Le compte des charges en marche sert l'avertissement : dix espaces arretes
// et une application vivante ne sont pas la meme situation.
func TestCeQuiTourneEstCompteApart(t *testing.T) {
	for _, etat := range []string{"", "stopped", "failed", "succeeded", "completed", "terminated", "deleted"} {
		if isRunningStatus(etat) {
			t.Errorf("%q compte comme en marche", etat)
		}
	}
	for _, etat := range []string{"running", "Running", "pending", "starting", "ready"} {
		if !isRunningStatus(etat) {
			t.Errorf("%q ne compte pas comme en marche", etat)
		}
	}
}
