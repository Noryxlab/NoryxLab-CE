package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Attacher une selection tiree de donnees regulees demande l'autorite
// plateforme.
//
// Depuis qu'un extrait se monte sans son dataset, attacher l'extrait livre ses
// octets. La decision a donc change de place : avant, atteindre une selection
// HDS exigeait un droit sur le dataset, et ce droit etait la decision; apres,
// l'attachement est la decision. Les deux sont gardes au meme niveau.
func fixtureAttache(t *testing.T, classification string) (Handlers, project.Project, extractdomain.Extract) {
	t.Helper()
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Projet", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "membre", access.RoleAdmin)

	datasets := memory.NewDatasetStore()
	jeu := datasetdomain.New("owner", "HDS-For-selena", "", "hds-for-selena", "", "s3", classification, "https://cellar", "")
	if err := datasets.Create(jeu); err != nil {
		t.Fatal(err)
	}
	ontologies := memory.NewOntologyObjectStore()
	onto := ontologydomain.New("owner", "Selena", "", "dataset", jeu.ID, jeu.Name, "health-file-path-v1", json.RawMessage(`{}`))
	if err := ontologies.Create(onto); err != nil {
		t.Fatal(err)
	}
	extraits := memory.NewExtractStore()
	extrait := extractdomain.Extract{ID: "e-1", OntologyID: onto.ID, Name: "selena_extract_mri",
		OwnerUserID: "membre", OwnerType: ownerUser, OwnerID: "membre"}
	if err := extraits.Create(extrait, nil); err != nil {
		t.Fatal(err)
	}
	return Handlers{
		authMode:             "header",
		projectStore:         projects,
		accessStore:          accessStore,
		datasetStore:         datasets,
		ontologyStore:        ontologies,
		extractStore:         extraits,
		projectResourceStore: memory.NewProjectResourceStore(),
		auditStore:           memory.NewAuditStore(),
	}, item, extrait
}

func attache(t *testing.T, h Handlers, projectID, extractID, utilisateur string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/extracts/"+extractID, nil)
	request.Header.Set("X-Noryx-User", utilisateur)
	request.SetPathValue("projectID", projectID)
	request.SetPathValue("extractID", extractID)
	recorder := httptest.NewRecorder()
	h.AttachProjectExtract(recorder, request)
	return recorder
}

func TestUnExtraitHdsNeSAttachePasSansAutoritePlateforme(t *testing.T) {
	h, item, extrait := fixtureAttache(t, "hds")

	reponse := attache(t, h, item.ID, extrait.ID, "membre")
	if reponse.Code != http.StatusForbidden {
		t.Fatalf("attendu 403, recu %d: %s", reponse.Code, reponse.Body.String())
	}
	// Et le refus dit qui peut le faire, pas seulement qu'il refuse.
	if !strings.Contains(reponse.Body.String(), "administrator") {
		t.Errorf("le refus doit nommer qui attache: %s", reponse.Body.String())
	}
	ids, _ := h.projectResourceStore.ListProjectExtractIDs(item.ID)
	if len(ids) != 0 {
		t.Fatalf("rien ne doit avoir ete attache: %v", ids)
	}
}

func TestUnExtraitHdsSAttachePourUnAdministrateur(t *testing.T) {
	h, item, extrait := fixtureAttache(t, "hds")
	h.bootstrapAdminUser = "patronne"

	if reponse := attache(t, h, item.ID, extrait.ID, "patronne"); reponse.Code != http.StatusOK {
		t.Fatalf("attendu 200, recu %d: %s", reponse.Code, reponse.Body.String())
	}
	ids, _ := h.projectResourceStore.ListProjectExtractIDs(item.ID)
	if len(ids) != 1 {
		t.Fatalf("l'extrait devait etre attache: %v", ids)
	}
}

// Et une selection ordinaire reste au niveau du projet.
func TestUnExtraitNonHdsSAttacheSansAdministrateur(t *testing.T) {
	h, item, extrait := fixtureAttache(t, "non-hds")

	if reponse := attache(t, h, item.ID, extrait.ID, "membre"); reponse.Code != http.StatusOK {
		t.Fatalf("attendu 200, recu %d: %s", reponse.Code, reponse.Body.String())
	}
}
