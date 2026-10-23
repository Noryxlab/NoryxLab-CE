package dataset

import (
	"encoding/json"
	"testing"
)

// La regle de SELENA, telle qu'elle est enregistree, lue sur de vrais chemins.
//
// Elle a ete posee a la main une fois, et le niveau de la modalite s'est
// retrouve au meme cran que le type de donnees : le scan a alors lu les
// dossiers comme des modalites, et l'etude a eu seize modalites nommees
// CORNEAMAPS, DICOM et SLN PDT1 0.5 CENTRE POCHED. Rien n'etait perdu - une
// lecture se refait - mais rien non plus ne l'avait refusee avant le scan.
func TestLaRegleDeSelenaLitLesQuatreNiveaux(t *testing.T) {
	var layout PathLayout
	stockee := `{"subjectLevel":1,"visitLevel":2,"modalityLevel":3,"dataTypeLevel":4,
	             "subjectName":"patient","visitName":"visite","modalityName":"modalite",
	             "dataTypeName":"type de donnees"}`
	if err := json.Unmarshal([]byte(stockee), &layout); err != nil {
		t.Fatal(err)
	}
	if probleme := layout.Validate(); probleme != "" {
		t.Fatalf("la regle enregistree doit etre valide : %s", probleme)
	}

	cas := []struct{ chemin, sujet, visite, modalite, typeDonnees string }{
		{"SELENA/SELENA-01-001/20260218/ANTERION/TabulatedData/SELENA-01-001_Cornea_Basics.csv",
			"SELENA-01-001", "20260218", "ANTERION", "TabulatedData"},
		{"SELENA/SELENA-01-005/20260422/ANTERION/DICOM/DICOM/MC27/26070/105568/00000656",
			"SELENA-01-005", "20260422", "ANTERION", "DICOM"},
		{"SELENA/SELENA-01-008/20260521/SPECTRALIS/SELENA-01-008-20260521.E2E",
			"SELENA-01-008", "20260521", "SPECTRALIS", ""},
		{"SELENA/SELENA-01-009/20260701/IRM/SLN PDT1 0.5 Centre PocheG/ACF1.rot.dcm",
			"SELENA-01-009", "20260701", "IRM", "SLN PDT1 0.5 Centre PocheG"},
	}
	for _, c := range cas {
		sujet, visite, modalite, typeDonnees := layout.Read(c.chemin)
		if sujet != c.sujet || visite != c.visite || modalite != c.modalite || typeDonnees != c.typeDonnees {
			t.Errorf("%s\n  lu     = %q %q %q %q\n  attendu= %q %q %q %q",
				c.chemin, sujet, visite, modalite, typeDonnees, c.sujet, c.visite, c.modalite, c.typeDonnees)
		}
	}

	// Et la regle qui a produit la mauvaise lecture est refusee.
	cassee := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 4, DataTypeLevel: 4}
	if cassee.Validate() == "" {
		t.Fatal("la modalite et le type de donnees au meme niveau doivent etre refuses")
	}
}
