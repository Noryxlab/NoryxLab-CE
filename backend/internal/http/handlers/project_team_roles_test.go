package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// L'appelant des tests est le jeton de service : il ne prend jamais l'identite
// d'une personne - un test du depot le verrouille explicitement - donc les
// droits sont accordes au nom qu'il porte reellement.
func requeteService(methode, chemin, _ string, corps io.Reader) *http.Request {
	requete := httptest.NewRequest(methode, chemin, corps)
	requete.Header.Set("X-Noryx-Service-Token", "s3cret")
	return requete
}

func nouveauProjetDeTest(t *testing.T, projets *memory.ProjectStore, proprietaire string) string {
	t.Helper()
	item := project.Project{ID: "projet-1", Name: "Essai",
		OwnerType: "user", OwnerID: proprietaire}
	if err := projets.Create(item); err != nil {
		t.Fatal(err)
	}
	return item.ID
}

// Une installation sans equipes n'a pas d'octrois, pas une erreur : un ecran
// qui les demande doit afficher une liste vide, pas un echec.
func TestSansEquipesLaListeEstVideEtPasEnErreur(t *testing.T) {
	projets := memory.NewProjectStore()
	projet := nouveauProjetDeTest(t, projets, "stef")
	acces := memory.NewAccessStore()
	acces.SetRole(projet, auth.ServiceUsername, access.RoleViewer)
	h := Handlers{projectStore: projets, accessStore: acces,
		serviceToken: "s3cret"} // teamStore nil

	enregistreur := httptest.NewRecorder()
	requete := requeteService(http.MethodGet,
		"/api/v1/projects/"+projet+"/team-roles", "stef", nil)
	requete.SetPathValue("projectID", projet)
	h.ListProjectTeamRoles(enregistreur, requete)

	if enregistreur.Code != http.StatusOK {
		t.Fatalf("code = %d, attendu 200 : %s", enregistreur.Code, enregistreur.Body.String())
	}
	var corps struct {
		Items []projectTeamRoleView `json:"items"`
	}
	_ = json.Unmarshal(enregistreur.Body.Bytes(), &corps)
	if len(corps.Items) != 0 {
		t.Fatalf("items = %+v, attendu vide", corps.Items)
	}
}

// Octroyer a une equipe qui n'existe pas creerait une ligne ouvrant le projet
// a un groupe qu'aucun ecran ne listera jamais - et elle survivrait a toute
// tentative ulterieure d'auditer qui atteint ce projet.
func TestOctroyerAUneEquipeInexistanteEstRefuse(t *testing.T) {
	projets := memory.NewProjectStore()
	projet := nouveauProjetDeTest(t, projets, "stef")
	equipes := memory.NewTeamStore()
	acces := memory.NewAccessStore()
	acces.SetRole(projet, auth.ServiceUsername, access.RoleAdmin)
	h := Handlers{projectStore: projets, accessStore: acces, teamStore: equipes,
		serviceToken: "s3cret"}

	enregistreur := httptest.NewRecorder()
	requete := requeteService(http.MethodPut,
		"/api/v1/projects/"+projet+"/team-roles/fantome", "stef",
		strings.NewReader(`{"role":"editor"}`))
	requete.SetPathValue("projectID", projet)
	requete.SetPathValue("teamID", "fantome")
	h.SetProjectTeamRole(enregistreur, requete)

	if enregistreur.Code != http.StatusNotFound {
		t.Fatalf("code = %d, attendu 404 : %s", enregistreur.Code, enregistreur.Body.String())
	}
	if octrois, _ := equipes.ListProjectRoles(projet); len(octrois) != 0 {
		t.Fatalf("un octroi fantome a ete ecrit : %+v", octrois)
	}
}

// Le chemin nominal, avec le nom d'equipe resolu : un tableau d'octrois qui
// montre des identifiants est un tableau qu'un administrateur ne peut pas
// auditer.
func TestUnOctroiValideRepondAvecLeNomDeLEquipe(t *testing.T) {
	projets := memory.NewProjectStore()
	projet := nouveauProjetDeTest(t, projets, "stef")
	equipes := memory.NewTeamStore()
	_ = equipes.Create(team.Team{ID: "t-pc", OrganizationID: "org-scor", Name: "P&C"})
	acces := memory.NewAccessStore()
	acces.SetRole(projet, auth.ServiceUsername, access.RoleAdmin)
	h := Handlers{projectStore: projets, accessStore: acces, teamStore: equipes,
		serviceToken: "s3cret"}

	enregistreur := httptest.NewRecorder()
	requete := requeteService(http.MethodPut,
		"/api/v1/projects/"+projet+"/team-roles/t-pc", "stef",
		strings.NewReader(`{"role":"editor"}`))
	requete.SetPathValue("projectID", projet)
	requete.SetPathValue("teamID", "t-pc")
	h.SetProjectTeamRole(enregistreur, requete)

	if enregistreur.Code != http.StatusOK {
		t.Fatalf("code = %d : %s", enregistreur.Code, enregistreur.Body.String())
	}
	var vue projectTeamRoleView
	_ = json.Unmarshal(enregistreur.Body.Bytes(), &vue)
	if vue.TeamName != "P&C" || vue.Role != "editor" {
		t.Fatalf("vue = %+v", vue)
	}
}
