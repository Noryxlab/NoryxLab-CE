package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// La matrice est ce qu'on lit pour repondre "qui peut ouvrir ce projet".
// Elle ne montrait que les octrois directs : un projet ouvert a une equipe
// entiere avait l'air de n'avoir personne.
func TestLaMatriceMontreLEquipeEtSesMembres(t *testing.T) {
	projets := memory.NewProjectStore()
	if err := projets.Create(project.Project{ID: "p1", Name: "Etude", OwnerType: "user", OwnerID: "stef"}); err != nil {
		t.Fatal(err)
	}
	equipes := memory.NewTeamStore()
	_ = equipes.Create(team.Team{ID: "t-pc", OrganizationID: "org-scor", Name: "P&C"})
	_ = equipes.AddMember("t-pc", "alice")
	_ = equipes.AddMember("t-pc", "bob")
	_ = equipes.SetProjectRole("p1", "t-pc", access.RoleEditor)

	h := Handlers{
		projectStore: projets, accessStore: memory.NewAccessStore(), teamStore: equipes,
		datasetStore: memory.NewDatasetStore(), ontologyStore: memory.NewOntologyObjectStore(),
		datasourceStore:      memory.NewDatasourceStore(),
		projectResourceStore: memory.NewProjectResourceStore(),
	}

	rapport, err := h.buildRBACMatrixReport()
	if err != nil {
		t.Fatal(err)
	}

	var equipeVue, viaEquipe int
	var nomEquipe string
	for _, sujet := range rapport.Subjects {
		if sujet.Type == "team" {
			equipeVue++
			nomEquipe = sujet.Name
		}
	}
	for _, cellule := range rapport.Cells {
		if cellule.Source == "team:P&C" {
			viaEquipe++
		}
	}

	if equipeVue != 1 {
		t.Errorf("%d sujets de type equipe, attendu 1 : c'est ce qu'un administrateur revoque", equipeVue)
	}
	if nomEquipe != "P&C" {
		t.Errorf("nom d'equipe = %q : la matrice montre un identifiant", nomEquipe)
	}
	if viaEquipe != 2 {
		t.Errorf("%d personnes atteintes par l'equipe, attendu 2", viaEquipe)
	}
}

// Une equipe n'est pas une personne. La version precedente du vocabulaire
// ramenait tout ce qui n'etait pas une organisation a "user", et la matrice
// aurait affiche un groupe dans la colonne des gens, sans erreur nulle part.
func TestUneEquipeNEstPasClasseeCommeUnePersonne(t *testing.T) {
	if got := normalizedSubjectType("team"); got != "team" {
		t.Fatalf("normalizedSubjectType(\"team\") = %q", got)
	}
	if got := normalizedSubjectType("inconnu"); got != "user" {
		t.Errorf("un type inconnu doit rester une personne, pas %q", got)
	}
}
