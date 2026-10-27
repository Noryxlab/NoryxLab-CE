package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
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

// Le graphe d'usage repond a "qui touche quelles donnees", ce qui pour un
// dataset regule est la question de conformite. Quelqu'un qui le lit par une
// equipe n'y figurait pas, et cette absence ressemblait exactement a ne pas y
// avoir acces.
func TestLeGrapheDUsageVoitLesMembresDEquipe(t *testing.T) {
	equipes := memory.NewTeamStore()
	_ = equipes.Create(team.Team{ID: "t-pc", OrganizationID: "org-scor", Name: "P&C"})
	_ = equipes.AddMember("t-pc", "alice")
	_ = equipes.SetProjectRole("p1", "t-pc", access.RoleViewer)
	h := Handlers{teamStore: equipes}

	atteints := teamMembersReaching(h, "p1")
	if len(atteints) != 1 || atteints[0].userID != "alice" || atteints[0].role != "viewer" {
		t.Fatalf("atteints = %+v", atteints)
	}

	// Sans magasin, le graphe se construit quand meme : un rapport qui refuse
	// de se batir parce qu'une arete manque ne sert personne.
	if atteints := teamMembersReaching(Handlers{}, "p1"); len(atteints) != 0 {
		t.Fatalf("sans magasin : %+v", atteints)
	}
}

// Un dataset attache a un projet n'est pas lu par les membres du projet : la
// lecture est un octroi sur l'objet (ADR-048). Le rapport ecrivait pourtant
// une ligne "project-editor" par membre et par dataset attache - des chemins
// presentes comme des droits. Et une equipe octroyee sur un dataset
// s'affichait par son identifiant, sans atteindre ses membres.
func TestLeRapportNeConfondPasAttacherEtLire(t *testing.T) {
	projets := memory.NewProjectStore()
	if err := projets.Create(project.Project{ID: "p1", Name: "Etude", OwnerType: "user", OwnerID: "stef"}); err != nil {
		t.Fatal(err)
	}
	acces := memory.NewAccessStore()
	acces.SetRole("p1", "bob", access.RoleEditor)
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(datasetdomain.Dataset{ID: "d1", Name: "HDS", OwnerType: "organization", OwnerID: "org-essilor"}); err != nil {
		t.Fatal(err)
	}
	attaches := memory.NewProjectResourceStore()
	_ = attaches.AttachDataset("p1", "d1")
	equipes := memory.NewTeamStore()
	_ = equipes.Create(team.Team{ID: "t-mri", OrganizationID: "org-essilor", Name: "MRI"})
	_ = equipes.AddMember("t-mri", "alice")
	if err := datasets.SetAccess(datasetdomain.Access{DatasetID: "d1", SubjectType: "team", SubjectID: "t-mri", Role: "writer"}); err != nil {
		t.Fatal(err)
	}

	h := Handlers{
		projectStore: projets, accessStore: acces, teamStore: equipes,
		datasetStore: datasets, ontologyStore: memory.NewOntologyObjectStore(),
		datasourceStore:      memory.NewDatasourceStore(),
		projectResourceStore: attaches,
	}
	rapport, err := h.buildRBACMatrixReport()
	if err != nil {
		t.Fatal(err)
	}
	var equipeNommee, aliceParEquipe bool
	for _, cellule := range rapport.Cells {
		if cellule.ResourceType == "dataset" && cellule.SubjectID == "bob" {
			t.Fatalf("bob, contributeur du projet, est donne sur le dataset attache : %+v", cellule)
		}
		if cellule.ResourceType == "dataset" && cellule.SubjectType == "team" && cellule.SubjectName == "MRI" {
			equipeNommee = true
		}
		if cellule.ResourceType == "dataset" && cellule.SubjectID == "alice" && cellule.Source == "team:MRI" && cellule.Inherited {
			aliceParEquipe = true
		}
	}
	if !equipeNommee {
		t.Error("l'equipe octroyee sur le dataset doit etre nommee, pas montree par son identifiant")
	}
	if !aliceParEquipe {
		t.Error("l'octroi a l'equipe doit atteindre ses membres, en herite")
	}
}
