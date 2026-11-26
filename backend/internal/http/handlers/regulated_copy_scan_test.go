package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/objectstore"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
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

// Le croisement d'empreintes, exerce de bout en bout.
//
// Jusqu'ici il ne pouvait pas l'etre : les handlers prenaient un client S3
// concret, donc la seule facon de voir le scan tourner etait d'avoir un
// dataset reglemente sur un cluster - ce que le DC n'a plus et ce que l'EMSE
// ne prete pas pour un test. internal/objectstore rend la chose testable ici,
// ce qui etait la moitie du point de l'abstraction.
func TestLeCroisementReconnaitUneCopieDansLesDeuxConfiances(t *testing.T) {
	datasets := memory.NewDatasetStore()
	regulated := dataset.Dataset{ID: "hds-1", Name: "cohorte", Bucket: "cellar-cohorte", Classification: "hds"}
	open := dataset.Dataset{ID: "open-1", Name: "perso", Bucket: "minio-perso"}
	for _, item := range []dataset.Dataset{regulated, open} {
		if err := datasets.Create(item); err != nil {
			t.Fatal(err)
		}
	}

	store := objectstore.NewMemory("cellar-cohorte", "minio-perso")
	// Petit : l'envoi tient en une partie, donc l'ETag est le MD5 et la copie
	// se reconnait avec certitude.
	store.Seed("cellar-cohorte", "sujet-01/notes.txt", []byte("donnee de sante"))
	store.Seed("minio-perso", "recup/notes.txt", []byte("donnee de sante"))
	// Gros, et recopie sous le meme nom avec un contenu identique : ici le
	// faux magasin rend le meme ETag, donc c'est encore `identical`. Le cas
	// multipart, ou les empreintes divergent, est couvert dans le domaine.
	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = byte(i % 251)
	}
	store.Seed("cellar-cohorte", "sujet-01/scan.nii", big)
	store.Seed("minio-perso", "ailleurs/scan.nii", big)
	// Et quelque chose qui n'a rien a voir : il ne doit pas etre nomme.
	store.Seed("minio-perso", "mes-notes.md", []byte("rien a voir"))

	h := Handlers{
		datasetStore:         datasets,
		projectResourceStore: fakeDatasetProjects{"hds-1": {"premyom"}, "open-1": {"premyom"}},
		datasetStores:        func(dataset.Dataset) (objectstore.Store, error) { return store, nil },
	}

	report := h.scanRegulatedCopies(context.Background())

	if report.PairsCompared != 1 {
		t.Fatalf("paires comparees = %d, attendu 1", report.PairsCompared)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("%d constat(s), attendu 1 : %+v", len(report.Findings), report.Findings)
	}
	finding := report.Findings[0]
	if finding.RegulatedDatasetName != "cohorte" || finding.OpenDatasetName != "perso" {
		t.Fatalf("mauvaise paire nommee : %+v", finding)
	}
	if finding.Identical+finding.Probable != 2 {
		t.Fatalf("2 objets reconnus attendus, obtenu %d identiques et %d probables",
			finding.Identical, finding.Probable)
	}
	for _, copied := range finding.Copies {
		if copied.OpenPath == "mes-notes.md" {
			t.Error("un objet sans rapport a ete nomme comme une copie")
		}
		// Les deux cotes nommes, et dans le bon sens : « trois objets
		// correspondent » envoie quelqu'un chercher, et une paire inversee
		// l'envoie chercher au mauvais endroit - le chemin reglemente vient
		// de la cohorte, l'autre du dataset non classe.
		if !strings.HasPrefix(copied.RegulatedPath, "sujet-01/") {
			t.Errorf("le chemin reglemente ne vient pas de la cohorte : %+v", copied)
		}
		if strings.HasPrefix(copied.OpenPath, "sujet-01/") {
			t.Errorf("les deux cotes sont inverses : %+v", copied)
		}
	}
}

// Le perimetre est celui de la regle : sans projet commun, aucun workload n'a
// pu monter les deux, et le scan ne paie pas le listage.
func TestSansProjetCommunLeCroisementNeListeRien(t *testing.T) {
	datasets := memory.NewDatasetStore()
	for _, item := range []dataset.Dataset{
		{ID: "hds-1", Name: "cohorte", Bucket: "a", Classification: "hds"},
		{ID: "open-1", Name: "perso", Bucket: "b"},
	} {
		if err := datasets.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	listed := false
	h := Handlers{
		datasetStore:         datasets,
		projectResourceStore: fakeDatasetProjects{"hds-1": {"premyom"}, "open-1": {"autre-projet"}},
		datasetStores: func(dataset.Dataset) (objectstore.Store, error) {
			listed = true
			return objectstore.NewMemory("a", "b"), nil
		},
	}

	report := h.scanRegulatedCopies(context.Background())

	if report.PairsCompared != 0 || len(report.Findings) != 0 {
		t.Fatalf("rien n'etait co-monte : %+v", report)
	}
	if listed {
		t.Error("le scan a ouvert un magasin alors qu'aucune paire ne le justifiait")
	}
}
