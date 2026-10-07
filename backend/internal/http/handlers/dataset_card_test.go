package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Le manifeste que le scan aurait produit sur hds-for : 32 sujets, 24 604
// objets, deux modalites, des visites datees.
func manifesteMesure(t *testing.T) json.RawMessage {
	t.Helper()
	m := ontologyManifest{
		SourceType: "dataset", SourceID: "ds-1",
		Summary:     ontologySummary{Subjects: 32, Objects: 24604},
		GeneratedAt: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC),
		Subjects: []ontologySubject{{
			ID: "AAAAAAA1234-5678",
			Visits: []ontologyVisit{
				{Date: "20240115", Modalities: []ontologyModality{{Name: "DICOM"}}},
				{Date: "20250630", Modalities: []ontologyModality{{Name: "OCT"}}},
			},
		}},
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func handlersAvecDataset(t *testing.T) (Handlers, *memory.DatasetStore) {
	t.Helper()
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(datasetdomain.Dataset{
		ID: "ds-1", Name: "hds-for", OwnerUserID: "stef", OwnerType: "user", OwnerID: "stef",
		Bucket: "hds-for", Classification: "regulated",
	}); err != nil {
		t.Fatal(err)
	}
	ontologies := memory.NewOntologyObjectStore()
	if err := ontologies.Create(ontology.Ontology{
		ID: "onto-1", Name: "hds-for", OwnerUserID: "stef", OwnerType: "user", OwnerID: "stef",
		SourceType: "dataset", SourceID: "ds-1", Manifest: manifesteMesure(t),
	}); err != nil {
		t.Fatal(err)
	}
	return Handlers{datasetStore: datasets, ontologyStore: ontologies}, datasets
}

// identifie attache l'identite par le sceau de contexte, comme ailleurs dans
// ces tests : le handler ne connait que requireIdentity.
func identifie(r *http.Request) *http.Request {
	return r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{Username: "stef"}))
}

func lis(t *testing.T, h Handlers, id string) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/"+id+"/card", nil)
	r.SetPathValue("datasetID", id)
	r = identifie(r)
	w := httptest.NewRecorder()
	h.GetDatasetCard(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code %d : %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func verdict(t *testing.T, payload map[string]any, champ string) map[string]any {
	t.Helper()
	for _, brut := range payload["checks"].([]any) {
		c := brut.(map[string]any)
		if c["field"] == champ {
			return c
		}
	}
	t.Fatalf("aucun controle pour %q", champ)
	return nil
}

// Un dataset que personne n'a decrit se lit quand meme, et dit ce qu'il mesure.
//
// C'est l'etat de depart de tous les datasets existants : la card est vide, et
// une card vide est un etat legitime qui se montre comme vide.
func TestUnDatasetSansCardSeLitEtMontreSesMesures(t *testing.T) {
	h, _ := handlersAvecDataset(t)
	payload := lis(t, h, "ds-1")
	if payload["declared"] != false {
		t.Fatalf("declared = %v, attendu false", payload["declared"])
	}
	if payload["measuredBy"] != "path scan" {
		t.Fatalf("measuredBy = %v, attendu \"path scan\"", payload["measuredBy"])
	}
	c := verdict(t, payload, "subjects")
	if c["verdict"] != "undeclared" || c["measured"] != "32" {
		t.Fatalf("subjects : %+v", c)
	}
}

// La declaration est ecrite, versionnee, attribuee - et confrontee.
func TestUneDeclarationEstVersionneeAttribueeEtConfrontee(t *testing.T) {
	h, _ := handlersAvecDataset(t)
	corps := `{"study":"SELENA","purpose":"mesurer la progression","subjects":32,
	           "objects":24604,"modalities":["DICOM","OCT"],"firstVisit":"20240115",
	           "pseudonymised":true}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/ds-1/card", strings.NewReader(corps))
	r.SetPathValue("datasetID", "ds-1")
	r = identifie(r)
	w := httptest.NewRecorder()
	h.SetDatasetCard(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code %d : %s", w.Code, w.Body.String())
	}

	payload := lis(t, h, "ds-1")
	card := payload["card"].(map[string]any)
	if card["version"] != float64(1) {
		t.Fatalf("version = %v, attendu 1", card["version"])
	}
	if card["declaredBy"] != "stef" {
		t.Fatalf("declaredBy = %v, attendu stef", card["declaredBy"])
	}
	// Les figures justes sont confirmees.
	for _, champ := range []string{"subjects", "objects", "modalities", "firstVisit"} {
		if c := verdict(t, payload, champ); c["verdict"] != "agrees" {
			t.Errorf("%s : %+v", champ, c)
		}
	}
	// La prose ne se verifie pas, et le dire est un verdict.
	if c := verdict(t, payload, "purpose"); c["verdict"] != "not_checkable" {
		t.Errorf("purpose : %+v", c)
	}
	// Et la pseudonymisation reste non verifiee : aucun scan de structure n'a
	// encore lu les champs techniques. C'est ce que l'enquete du 05/10/2026
	// n'avait nulle part pour ecrire.
	c := verdict(t, payload, "pseudonymised")
	if c["verdict"] != "unverified" {
		t.Fatalf("pseudonymised : %+v", c)
	}
	if c["note"] == "" {
		t.Fatal("il faut dire pourquoi ce n'est pas verifie")
	}

	// Une seconde edition incremente, elle ne repart pas de un.
	r2 := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/ds-1/card", strings.NewReader(`{"study":"SELENA v2"}`))
	r2.SetPathValue("datasetID", "ds-1")
	r2 = identifie(r2)
	w2 := httptest.NewRecorder()
	h.SetDatasetCard(w2, r2)
	if v := lis(t, h, "ds-1")["card"].(map[string]any)["version"]; v != float64(2) {
		t.Fatalf("version apres seconde edition = %v, attendu 2", v)
	}
}

// Une declaration fausse est rapportee et conservee telle quelle.
func TestUneDeclarationFausseEstRapporteeEtConservee(t *testing.T) {
	h, _ := handlersAvecDataset(t)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/ds-1/card",
		strings.NewReader(`{"subjects":34,"modalities":["DICOM"]}`))
	r.SetPathValue("datasetID", "ds-1")
	r = identifie(r)
	h.SetDatasetCard(httptest.NewRecorder(), r)

	payload := lis(t, h, "ds-1")
	c := verdict(t, payload, "subjects")
	if c["verdict"] != "differs" || c["declared"] != "34" || c["measured"] != "32" {
		t.Fatalf("subjects : %+v", c)
	}
	// La declaration n'a pas ete reecrite par la mesure.
	if claims := payload["card"].(map[string]any)["claims"].(map[string]any); claims["subjects"] != float64(34) {
		t.Fatalf("la declaration a ete reecrite : %+v", claims)
	}
	// Et l'ecart d'ensemble dit dans quel sens.
	if c := verdict(t, payload, "modalities"); c["verdict"] != "differs" || c["note"] == "" {
		t.Fatalf("modalities : %+v", c)
	}
}

// Un corps vide efface la card : c'est la sortie quand une declaration s'avere
// fausse et que personne ne sait encore par quoi la remplacer.
func TestUnCorpsVideEffaceLaCard(t *testing.T) {
	h, _ := handlersAvecDataset(t)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/ds-1/card", strings.NewReader(`{"study":"SELENA"}`))
	r.SetPathValue("datasetID", "ds-1")
	r = identifie(r)
	h.SetDatasetCard(httptest.NewRecorder(), r)

	r2 := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/ds-1/card", strings.NewReader(`{}`))
	r2.SetPathValue("datasetID", "ds-1")
	r2 = identifie(r2)
	w2 := httptest.NewRecorder()
	h.SetDatasetCard(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("code %d", w2.Code)
	}
	if lis(t, h, "ds-1")["declared"] != false {
		t.Fatal("la card n'a pas ete effacee")
	}
}
