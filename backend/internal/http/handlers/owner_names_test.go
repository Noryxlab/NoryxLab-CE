package handlers

import (
	"testing"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
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
