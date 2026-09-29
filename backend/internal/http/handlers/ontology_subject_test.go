package handlers

import "testing"

// Les identifiants tels que les etudes les ecrivent.
//
// La regle etait un tiret et un nombre : PREMYOM1000-0001 passait,
// SELENA-01-001 non, parce que la partie alphanumerique ne traverse pas un
// tiret. FOR numerote les patients SELENA par centre, donc la chaine entiere
// est le patient - et un balayage de cette etude ne reconnaissait aucun sujet,
// produisant une ontologie vide qui avait l'air d'avoir marche.
func TestUnSujetSeReconnaitQuelQueSoitSonDecoupage(t *testing.T) {
	for _, identifiant := range []string{
		"PREMYOM1000-0001", "SELENA-01-001", "SELENA-01-005",
		"VERCORS-2-014", "A-001", "ETUDE-01-02-0003",
	} {
		if !ontologySubjectPattern.MatchString(identifiant) {
			t.Errorf("%q n'est pas reconnu comme sujet", identifiant)
		}
	}
}

// Ce qui n'est pas relache : un sujet finit par trois ou quatre chiffres.
// C'est ce qui empeche un repertoire de passer pour un patient, et sans quoi
// le balayage rangerait les donnees sous "DICOM".
func TestUnRepertoireNEstPasUnSujet(t *testing.T) {
	for _, chemin := range []string{
		"DICOM", "modality_ANTERION", "ANTERION", "visit_20250430", "20250430",
		"MC22", "old", "checksums", "SELENA", "PREMYOM1000",
		"serie-401-outputs-final", "patient-12345",
	} {
		if ontologySubjectPattern.MatchString(chemin) {
			t.Errorf("%q est lu comme un sujet alors que c'est un repertoire", chemin)
		}
	}
}

// Les trois positions restent les memes : sujet, puis visite, puis modalite.
func TestLesTroisPositionsSeLisentEnsemble(t *testing.T) {
	cas := []struct{ chemin, sujet, visite, modalite string }{
		{"SELENA/SELENA-01-001/20260218/modality_ANTERION/a.dcm", "SELENA-01-001", "20260218", "ANTERION"},
		{"PREMYOM1000/PREMYOM1000-0001/visit_20250430/ANTERION/x", "PREMYOM1000-0001", "20250430", "ANTERION"},
		{"checksums/liste.txt", "", "", ""},
	}
	for _, item := range cas {
		sujet, visite, modalite := inferOntologyPath(item.chemin)
		if sujet != item.sujet || visite != item.visite || modalite != item.modalite {
			t.Errorf("%q -> (%q, %q, %q), attendu (%q, %q, %q)",
				item.chemin, sujet, visite, modalite, item.sujet, item.visite, item.modalite)
		}
	}
}
