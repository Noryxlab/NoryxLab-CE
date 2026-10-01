package handlers

import (
	"testing"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A person is stored by their username, which reads perfectly well; an
// organization is stored by an identifier, which reads as nothing at all. When
// the directory cannot be reached the identifier is shown - a page that fails
// because a name could not be resolved is worse than a page showing the id.
func TestOwnerNamesFallBackToTheIdentifier(t *testing.T) {
	h := Handlers{}

	datasets := []datasetdomain.Dataset{
		{OwnerType: "user", OwnerID: "stef"},
		{OwnerType: "organization", OwnerID: "0f2c8a1e-4b1d-4a77-9a1f-2c7c9f0a1b23"},
	}
	h.nameDatasetOwners(datasets)
	if datasets[0].OwnerName != "stef" {
		t.Fatalf("user owner name = %q, want stef", datasets[0].OwnerName)
	}
	if datasets[1].OwnerName != datasets[1].OwnerID {
		t.Fatalf("unresolved organization = %q, want the identifier", datasets[1].OwnerName)
	}

	// The same notation has to reach ontologies, or the same organization reads
	// one way on one screen and another way on the next.
	ontologies := []ontologydomain.Ontology{{OwnerType: "user", OwnerID: "stef"}}
	h.nameOntologyOwners(ontologies)
	if ontologies[0].OwnerName != "stef" {
		t.Fatalf("ontology owner name = %q, want stef", ontologies[0].OwnerName)
	}
}

// Teams became owners alongside people and organizations, and a team is stored
// by identifier like an organization is. Without resolution the catalogue read
// "b4e1..." in the owner column of every object handed to a team - which is
// the notation this file exists to prevent.
func TestATeamOwnerReadsAsItsName(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "t-pc", OrganizationID: "org-scor", Name: "P&C"}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{teamStore: equipes}

	extraits := []extractdomain.Extract{
		{OwnerType: ownerTeam, OwnerID: "t-pc"},
		{OwnerType: ownerTeam, OwnerID: "t-disparue"},
		{OwnerType: ownerUser, OwnerID: "stef"},
	}
	h.nameExtractOwners(extraits)
	if extraits[0].OwnerName != "P&C" {
		t.Fatalf("team owner name = %q, want P&C", extraits[0].OwnerName)
	}
	// Une equipe supprimee ne casse pas la page : on retombe sur l'identifiant.
	if extraits[1].OwnerName != "t-disparue" {
		t.Fatalf("unknown team = %q, want the identifier", extraits[1].OwnerName)
	}
	if extraits[2].OwnerName != "stef" {
		t.Fatalf("user owner name = %q, want stef", extraits[2].OwnerName)
	}

	// Meme notation sur la source de donnees, sinon la meme equipe se lit
	// d'une facon dans un onglet du catalogue et d'une autre dans le suivant.
	sources := []datasourcedomain.Datasource{{OwnerType: ownerTeam, OwnerID: "t-pc"}}
	h.nameDatasourceOwners(sources)
	if sources[0].OwnerName != "P&C" {
		t.Fatalf("datasource team owner = %q, want P&C", sources[0].OwnerName)
	}
}

// Un store d'equipes absent est le cas de l'installation sans equipes : on
// affiche l'identifiant, on ne tombe pas.
func TestATeamOwnerSurvivesTheAbsenceOfATeamStore(t *testing.T) {
	h := Handlers{}
	extraits := []extractdomain.Extract{{OwnerType: ownerTeam, OwnerID: "t-pc"}}
	h.nameExtractOwners(extraits)
	if extraits[0].OwnerName != "t-pc" {
		t.Fatalf("owner name = %q, want the identifier", extraits[0].OwnerName)
	}
}
