package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Une app redemarree remonte son manifeste d'extraits.
//
// Le script d'une app est reutilise tel quel au redemarrage - le record garde
// le script, pas l'argv tape - et ce script teste la presence de
// /var/run/noryx/bootstrap/extracts.b64 avant de construire quoi que ce soit.
// La relance montait les volumes de dataset et oubliait ce secret : le test
// echouait sans bruit, et une application qui avait un arbre d'extraits
// revenait sans. Le pod etait sain, /extracts simplement vide.
//
// C'est le meme oubli que pour les jobs, et le meme indice : un secret cree a
// la naissance de la charge et lu par personne ensuite. Les tests existants
// portaient sur le script, qui etait juste; celui-ci porte sur ce que le
// runtime recoit.
func TestUneAppRedemarreeRemonteSonManifesteDExtraits(t *testing.T) {
	h, runner, record := restartFixture(t, "running")

	// Un dataset attache au projet, et un extrait qui pointe dedans.
	jeu := datasetdomain.New("member", "verif-dataset", "", "noryx-ds-verif", "", "minio", "", "", "")
	if err := h.datasetStore.Create(jeu); err != nil {
		t.Fatal(err)
	}
	if err := h.projectResourceStore.AttachDataset(record.ProjectID, jeu.ID); err != nil {
		t.Fatal(err)
	}

	ontologies := memory.NewOntologyObjectStore()
	// SourceName est ce que le montage compare au nom du dataset attache.
	onto := ontologydomain.New("member", "Anterion", "", "dataset", jeu.ID, jeu.Name, "health-file-path-v1", json.RawMessage(`{}`))
	if err := ontologies.Create(onto); err != nil {
		t.Fatal(err)
	}
	h.ontologyStore = ontologies

	extraits := memory.NewExtractStore()
	extrait := extractdomain.Extract{
		ID: "e-anterion", OntologyID: onto.ID, Name: "anterion-3-sujets",
		OwnerUserID: "member", OwnerType: ownerUser, OwnerID: "member",
	}
	membres := []extractdomain.Member{
		{ExtractID: extrait.ID, Path: "SUJ-01/20260218/scan.dcm", SubjectID: "SUJ-01", Visit: "20260218", Modality: "ANTERION"},
	}
	if err := extraits.Create(extrait, membres); err != nil {
		t.Fatal(err)
	}
	h.extractStore = extraits
	if err := h.projectResourceStore.AttachExtract(record.ProjectID, extrait.ID); err != nil {
		t.Fatal(err)
	}

	if response := restart(t, h, "/api/v1/apps/"+record.ID+"/restart", record.ID); response.Code != http.StatusAccepted {
		t.Fatalf("restart = %d, want 202: %s", response.Code, response.Body.String())
	}
	if len(runner.pods) != 1 {
		t.Fatalf("la relance n'est pas passee par le chemin de lancement: %d pod(s)", len(runner.pods))
	}

	monte := false
	for _, mount := range runner.pods[0].Secrets {
		if mount.MountPath == "/var/run/noryx/bootstrap" {
			monte = true
		}
	}
	if !monte {
		t.Fatalf("le manifeste d'extraits doit etre monte sur une app relancee, monts recus: %+v", runner.pods[0].Secrets)
	}
}
