package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Le manifeste qu'un scan aurait produit : 32 sujets, 24 604 objets.
func manifesteMesure(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(ontologyManifest{
		SourceType: "dataset", SourceID: "ds-1",
		Summary:     ontologySummary{Subjects: 32, Objects: 24604},
		GeneratedAt: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC),
		Subjects: []ontologySubject{{
			ID:     "AAAAAAA1234-5678",
			Visits: []ontologyVisit{{Date: "20240115", Modalities: []ontologyModality{{Name: "DICOM"}}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func handlersAvecOntologie(t *testing.T) Handlers {
	t.Helper()
	store := memory.NewOntologyObjectStore()
	if err := store.Create(ontology.Ontology{
		ID: "onto-1", Name: "PREMYOM1000", OwnerUserID: "stef", OwnerType: "user", OwnerID: "stef",
		SourceType: "dataset", SourceID: "ds-1", Manifest: manifesteMesure(t),
	}); err != nil {
		t.Fatal(err)
	}
	return Handlers{ontologyStore: store}
}

func identifie(r *http.Request) *http.Request {
	return r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{Username: "stef"}))
}

func lisCard(t *testing.T, h Handlers) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/ontologies/onto-1/card", nil)
	r.SetPathValue("ontologyID", "onto-1")
	w := httptest.NewRecorder()
	h.GetOntologyCard(w, identifie(r))
	if w.Code != http.StatusOK {
		t.Fatalf("code %d : %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func ecrisCard(t *testing.T, h Handlers, corps string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/ontologies/onto-1/card", strings.NewReader(corps))
	r.SetPathValue("ontologyID", "onto-1")
	w := httptest.NewRecorder()
	h.SetOntologyCard(w, identifie(r))
	return w
}

// Une ontologie que personne n'a decrite se lit quand meme, et dit ce qu'elle
// mesure. C'est l'etat de depart de toutes les ontologies existantes.
func TestUneOntologieSansCardSeLitEtMontreSesMesures(t *testing.T) {
	payload := lisCard(t, handlersAvecOntologie(t))
	if payload["declared"] != false {
		t.Fatalf("declared = %v, attendu false", payload["declared"])
	}
	mesure := payload["measured"].(map[string]any)
	if mesure["method"] != "path scan" || mesure["subjects"] != float64(32) {
		t.Fatalf("les mesures doivent rester visibles sans declaration : %+v", mesure)
	}
	// Et rien n'a regarde dans les fichiers.
	if payload["structure"].(map[string]any)["ran"] != false {
		t.Fatalf("structure : %+v", payload["structure"])
	}
}

// Un paragraphe, versionne et attribue - et les mesures restent a cote.
//
// Un seul champ, parce qu'un formulaire de quatorze champs etiquetes est un
// formulaire que personne ne remplit, et parce que la moitie de ces etiquettes
// etait le vocabulaire d'un hopital sur une plateforme vendue aussi a des
// banques.
func TestUnParagrapheEstVersionneEtAttribue(t *testing.T) {
	h := handlersAvecOntologie(t)
	if w := ecrisCard(t, h, `{"text":"Cohorte PREMYOM1000. Ne pas utiliser pour du depistage."}`); w.Code != http.StatusOK {
		t.Fatalf("code %d : %s", w.Code, w.Body.String())
	}
	payload := lisCard(t, h)
	card := payload["card"].(map[string]any)
	if card["version"] != float64(1) || card["declaredBy"] != "stef" {
		t.Fatalf("card = %+v", card)
	}
	if !strings.Contains(card["text"].(string), "depistage") {
		t.Fatalf("le texte n'a pas ete garde : %+v", card)
	}
	// Les mesures voyagent toujours avec la declaration : une card qui ne
	// repete que ce qu'on lui a dit est une brochure.
	if payload["measured"].(map[string]any)["objects"] != float64(24604) {
		t.Fatalf("mesures = %+v", payload["measured"])
	}

	if w := ecrisCard(t, h, `{"text":"Deuxieme version."}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if v := lisCard(t, h)["card"].(map[string]any)["version"]; v != float64(2) {
		t.Fatalf("version = %v, attendu 2", v)
	}
}

// Un texte vide efface la card : c'est la sortie quand une description s'avere
// fausse et que personne ne sait encore par quoi la remplacer.
func TestUnTexteVideEffaceLaCard(t *testing.T) {
	h := handlersAvecOntologie(t)
	ecrisCard(t, h, `{"text":"quelque chose"}`)
	if w := ecrisCard(t, h, `{"text":"   "}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if lisCard(t, h)["declared"] != false {
		t.Fatal("la card n'a pas ete effacee")
	}
}

// Le scan de structure se montre a cote, avec sa date et les champs regardes.
func TestLeScanDeStructureSeMontreAvecSaDate(t *testing.T) {
	h := handlersAvecOntologie(t)
	corps := `{"allowlists":[{"format":"dicom","record":["Modality"],
	            "identifying":["PatientName","PatientID"]}],
	           "objects":3592,"identifyingChecked":["PatientName","PatientID"],
	           "identifyingPresent":["PatientName"],
	           "method":"structure scan (pydicom)"}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/ontologies/onto-1/structure-scan", strings.NewReader(corps))
	r.SetPathValue("ontologyID", "onto-1")
	w := httptest.NewRecorder()
	h.SetOntologyStructureScan(w, identifie(r))
	if w.Code != http.StatusOK {
		t.Fatalf("code %d : %s", w.Code, w.Body.String())
	}

	structure := lisCard(t, h)["structure"].(map[string]any)
	if structure["ran"] != true || structure["identifiersPresent"] != true {
		t.Fatalf("structure = %+v", structure)
	}
	if structure["method"] != "structure scan (pydicom)" || structure["at"] == "" {
		t.Fatalf("la methode et la date doivent etre nommees : %+v", structure)
	}
	present := structure["present"].([]any)
	if len(present) != 1 || present[0] != "PatientName" {
		t.Fatalf("le champ fautif doit etre nomme, et lui seul : %+v", present)
	}
}

// Un scanner ne peut pas enregistrer les valeurs d'un champ identifiant : il
// est un client, ce qu'il envoie ne decide pas.
func TestUnScannerNePeutPasEnregistrerUnChampIdentifiant(t *testing.T) {
	h := handlersAvecOntologie(t)
	corps := `{"allowlists":[{"format":"dicom","record":["Modality"],"identifying":["PatientName"]}],
	           "objects":10,
	           "tallies":[{"field":"PatientName","values":{"DUPONT":3},"distinct":1,"present":3}]}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/ontologies/onto-1/structure-scan", strings.NewReader(corps))
	r.SetPathValue("ontologyID", "onto-1")
	w := httptest.NewRecorder()
	h.SetOntologyStructureScan(w, identifie(r))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "PatientName") {
		t.Fatalf("code %d : %s", w.Code, w.Body.String())
	}
}
