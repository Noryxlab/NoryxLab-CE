package dataset

import "testing"

// La regle positionnelle lit les deux etudes qui existent, et c'est le point.
//
// PREMYOM1000 est range <sujet>/<visite>/<modalite>, SELENA ajoute un niveau
// d'etude devant. La regle compilee en dur trouvait la premiere en cherchant
// un segment qui ressemble a un identifiant ; la seconde etude a numerote ses
// patients autrement et il a fallu du Go. Une regle ecrite par dataset suffit.
func TestLaRegleLitLesDeuxDispositions(t *testing.T) {
	premyom := PathLayout{SubjectLevel: 0, VisitLevel: 1, ModalityLevel: 2}
	sujet, visite, modalite := premyom.Read("PREMYOM1000-0023/20260218/ANTERION/scan.dcm")
	if sujet != "PREMYOM1000-0023" || visite != "20260218" || modalite != "ANTERION" {
		t.Fatalf("premyom = %q %q %q", sujet, visite, modalite)
	}

	selena := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}
	sujet, visite, modalite = selena.Read("SELENA/SELENA-01-001/20260218/ANTERION/DICOM/MC27/f.dcm")
	if sujet != "SELENA-01-001" || visite != "20260218" || modalite != "ANTERION" {
		t.Fatalf("selena = %q %q %q", sujet, visite, modalite)
	}
}

// Les niveaux absents sont une reponse, pas un trou.
//
// Une etude a une visite par patient, une autre un seul instrument : dire
// "absent" est plus vrai que designer un segment qui veut dire autre chose.
func TestUnNiveauAbsentEstUneReponse(t *testing.T) {
	regle := PathLayout{SubjectLevel: 0, VisitLevel: LevelAbsent, ModalityLevel: 1}
	sujet, visite, modalite := regle.Read("P-001/ANTERION/scan.dcm")
	if sujet != "P-001" || visite != "" || modalite != "ANTERION" {
		t.Fatalf("lu = %q %q %q", sujet, visite, modalite)
	}
}

// Un chemin trop court ne produit pas de sujet : le scan le comptera comme non
// reconnu et decrira sa forme, au lieu de le ranger quelque part de plausible.
func TestUnCheminTropCourtNeProduitRien(t *testing.T) {
	regle := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}
	if sujet, _, _ := regle.Read("fichier.csv"); sujet != "" {
		t.Fatalf("sujet = %q, attendu aucun", sujet)
	}
	// Et une regle qui designe le fichier lui-meme ne fait pas de chaque
	// fichier un sujet - c'est ce qui produirait "4 026 sujets".
	surLeFichier := PathLayout{SubjectLevel: 3, VisitLevel: LevelAbsent, ModalityLevel: LevelAbsent}
	if sujet, _, _ := surLeFichier.Read("a/b/c/fichier.csv"); sujet != "" {
		t.Fatalf("sujet = %q : le dernier segment est le fichier", sujet)
	}
}

// Une regle incoherente est refusee, jamais corrigee.
//
// Une disposition ajustee en silence rangerait les donnees de quelqu'un sous
// un segment qu'il n'a pas choisi, et l'erreur ressortirait en un compte de
// sujets que personne ne sait expliquer.
func TestUneRegleIncoherenteEstRefusee(t *testing.T) {
	for _, essai := range []PathLayout{
		{SubjectLevel: LevelAbsent},
		{SubjectLevel: 1, VisitLevel: 1},
		{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 2},
		{SubjectLevel: 99},
	} {
		if probleme := essai.Validate(); probleme == "" {
			t.Fatalf("%+v aurait du etre refusee", essai)
		}
	}
	bonne := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}
	if probleme := bonne.Validate(); probleme != "" {
		t.Fatalf("regle valide refusee : %s", probleme)
	}
}

// Et elle se lit en francais sur un ecran, parce que c'est ainsi qu'on la
// verifie contre un chemin qu'on connait.
func TestLaRegleSeDecrit(t *testing.T) {
	regle := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}
	if regle.Describe() != "level 1 = subject, level 2 = visit, level 3 = modality" {
		t.Fatalf("description = %q", regle.Describe())
	}
	absente := PathLayout{SubjectLevel: LevelAbsent}
	if absente.Declared() {
		t.Fatal("une regle sans sujet n'est pas declaree")
	}
}
