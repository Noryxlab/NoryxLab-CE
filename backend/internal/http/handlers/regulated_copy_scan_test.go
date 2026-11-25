package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

func TestSeulsLesDatasetsLisiblesEntrentDansLeCroisement(t *testing.T) {
	regulated, open := splitByRegulation([]dataset.Dataset{
		{ID: "a", Name: "hds-for", Bucket: "cellar-for", Classification: "hds"},
		{ID: "b", Name: "HDS-majuscules", Bucket: "cellar-b", Classification: "HDS"},
		{ID: "c", Name: "dataset-local-stef", Bucket: "noryx-stef"},
		{ID: "d", Name: "declare-sans-bucket", Classification: "hds"},
		{ID: "e", Name: "ouvert-sans-bucket"},
	})
	if len(regulated) != 2 || len(open) != 1 {
		t.Fatalf("attendu 2 reglementes et 1 non classe, obtenu %d et %d", len(regulated), len(open))
	}
	if open[0].ID != "c" {
		t.Fatalf("le dataset non classe lisible devait etre c, obtenu %s", open[0].ID)
	}
}

// Le perimetre du scan est celui de la regle : les paires qui partagent un
// projet, donc celles qu'un workload a pu monter ensemble. Deux datasets qui
// ne se croisent dans aucun projet ne se comparent pas, et c'est ce qui rend
// le scan payable.
func TestLesPairesSontCellesQuUnWorkloadAPuMonterEnsemble(t *testing.T) {
	h := Handlers{projectResourceStore: fakeDatasetProjects{
		"hds-1":   {"premyom"},
		"hds-2":   {"essilor"},
		"open-1":  {"premyom", "autre"},
		"open-2":  {"sans-rapport"},
		"orpheHS": {},
	}}
	pairs := h.coMountedPairs(
		[]dataset.Dataset{{ID: "hds-1"}, {ID: "hds-2"}, {ID: "orpheHS"}},
		[]dataset.Dataset{{ID: "open-1"}, {ID: "open-2"}},
	)
	if len(pairs) != 1 {
		t.Fatalf("une seule paire co-montee attendue, obtenu %+v", pairs)
	}
	if pairs[0].regulated.ID != "hds-1" || pairs[0].open.ID != "open-1" {
		t.Fatalf("mauvaise paire: %+v", pairs[0])
	}
	if pairs[0].projectID != "premyom" {
		t.Fatalf("le projet partage doit etre nomme dans l'alerte, obtenu %q", pairs[0].projectID)
	}
}

type fakeDatasetProjects map[string][]string

func (f fakeDatasetProjects) ListDatasetProjectIDs(datasetID string) ([]string, error) {
	return f[datasetID], nil
}

func (f fakeDatasetProjects) AttachDataset(string, string) error             { return nil }
func (f fakeDatasetProjects) DetachDataset(string, string) error             { return nil }
func (f fakeDatasetProjects) ListProjectDatasetIDs(string) ([]string, error) { return nil, nil }
func (f fakeDatasetProjects) AttachRepository(string, string) error          { return nil }
func (f fakeDatasetProjects) DetachRepository(string, string) error          { return nil }
func (f fakeDatasetProjects) ListProjectRepositoryIDs(string) ([]string, error) {
	return nil, nil
}
func (f fakeDatasetProjects) AttachDatasource(string, string) error { return nil }
func (f fakeDatasetProjects) DetachDatasource(string, string) error { return nil }
func (f fakeDatasetProjects) ListProjectDatasourceIDs(string) ([]string, error) {
	return nil, nil
}
func (f fakeDatasetProjects) ListDatasourceProjectIDs(string) ([]string, error) {
	return nil, nil
}
func (f fakeDatasetProjects) AttachOntology(string, string) error             { return nil }
func (f fakeDatasetProjects) DetachOntology(string, string) error             { return nil }
func (f fakeDatasetProjects) ListProjectOntologyIDs(string) ([]string, error) { return nil, nil }
func (f fakeDatasetProjects) AttachExtract(string, string) error              { return nil }
func (f fakeDatasetProjects) DetachExtract(string, string) error              { return nil }
func (f fakeDatasetProjects) ListProjectExtractIDs(string) ([]string, error)  { return nil, nil }
func (f fakeDatasetProjects) ListExtractProjectIDs(string) ([]string, error)  { return nil, nil }
func (f fakeDatasetProjects) ListOntologyProjectIDs(string) ([]string, error) { return nil, nil }
