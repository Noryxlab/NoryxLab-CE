package ontology

import (
	"strings"
	"testing"
)

// Un champ dans les deux listes est l'erreur que cette verification existe pour
// attraper : il serait lu pour etre verifie *et* lu pour etre garde, et le
// second gagne par accident. Sur un bucket regule, c'est ainsi qu'un
// identifiant finit dans une card.
func TestUnChampDansLesDeuxListesEstRefuse(t *testing.T) {
	liste := StructureAllowlist{
		Format:      "dicom",
		Record:      []string{"Modality", "PatientID"},
		Identifying: []string{"PatientName", "patientid"},
	}
	probleme := liste.Problem()
	if probleme == "" {
		t.Fatal("un champ a la fois enregistre et identifiant a ete accepte")
	}
	if !strings.Contains(probleme, "PatientID") {
		t.Fatalf("le message doit nommer le champ : %q", probleme)
	}
}

// Une liste coherente passe ; une liste vide ne lit rien et le dit.
func TestUneListeCoherentePasseEtUneListeVideLeDit(t *testing.T) {
	bonne := StructureAllowlist{Format: "dicom",
		Record:      []string{"Modality", "Rows", "Columns"},
		Identifying: []string{"PatientName", "PatientID", "PatientBirthDate"}}
	if probleme := bonne.Problem(); probleme != "" {
		t.Fatalf("liste refusee : %s", probleme)
	}
	if probleme := (StructureAllowlist{Format: "dicom"}).Problem(); probleme == "" {
		t.Fatal("une liste qui ne nomme aucun champ a ete acceptee")
	}
	if probleme := (StructureAllowlist{Record: []string{"Modality"}}).Problem(); probleme == "" {
		t.Fatal("une liste sans format a ete acceptee")
	}
}

// La question "ce dataset est-il pseudonymise" a trois reponses, et la
// troisieme est celle qui compte : rien n'a ete verifie.
func TestLaPseudonymisationATroisReponses(t *testing.T) {
	if (*StructureScan)(nil).CarriedIdentifiers() != nil {
		t.Fatal("sans scan, la reponse doit etre inconnue")
	}
	vide := &StructureScan{Objects: 3592}
	if vide.CarriedIdentifiers() != nil {
		t.Fatal("un scan qui n'a verifie aucun champ ne repond pas")
	}
	propre := &StructureScan{IdentifyingChecked: []string{"PatientName", "PatientID"}}
	if got := propre.CarriedIdentifiers(); got == nil || *got {
		t.Fatalf("verifie et rien trouve : attendu false, obtenu %v", got)
	}
	sale := &StructureScan{
		IdentifyingChecked: []string{"PatientName", "PatientID"},
		IdentifyingPresent: []string{"PatientName"},
	}
	if got := sale.CarriedIdentifiers(); got == nil || !*got {
		t.Fatalf("un identifiant trouve : attendu true, obtenu %v", got)
	}
}

// Un champ a forte cardinalite perd ses valeurs, meme si le scanner les envoie.
//
// C'est la ligne entre une distribution et une colonne : une modalite a une
// poignee de valeurs, une date de naissance en a une par sujet, et une
// "distribution" dessus est la colonne avec des etapes en plus. Le scanner est
// un client ; ce qu'il envoie ne decide pas.
func TestUnChampAForteCardinalitePerdSesValeurs(t *testing.T) {
	valeurs := map[string]int{}
	for i := 0; i < StructureTallyCap+1; i++ {
		valeurs[string(rune('a'+i%26))+string(rune('0'+i/26))] = 1
	}
	scan := &StructureScan{Tallies: []FieldTally{
		{Field: "PatientBirthDate", Values: valeurs},
		{Field: "Modality", Values: map[string]int{"OP": 3588, "OCT": 4}},
	}}
	scan.Normalise()
	for _, tally := range scan.Tallies {
		switch tally.Field {
		case "PatientBirthDate":
			if tally.Values != nil || !tally.Suppressed {
				t.Fatalf("les valeurs ont ete gardees : %+v", tally)
			}
			if tally.Distinct != StructureTallyCap+1 {
				t.Fatalf("le nombre de valeurs distinctes doit survivre : %d", tally.Distinct)
			}
		case "Modality":
			if tally.Suppressed || len(tally.Values) != 2 {
				t.Fatalf("une vraie distribution a ete supprimee : %+v", tally)
			}
		}
	}
}
