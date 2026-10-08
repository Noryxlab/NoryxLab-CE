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

// Et elle se lit sur un ecran, parce que c'est ainsi qu'on la verifie contre un
// chemin qu'on connait - dans les mots du metier qui possede la donnee.
//
// Sans noms declares, les defauts sont generiques et non cliniques. Les
// positions etaient configurables et les mots ne l'etaient pas, donc une
// plateforme vendue a des banques affichait « sujet », « visite » et
// « modalite » sur une table d'operations.
func TestLaRegleSeDecrit(t *testing.T) {
	regle := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}
	if regle.Describe() != "level 1 = entity, level 2 = period, level 3 = category" {
		t.Fatalf("description = %q", regle.Describe())
	}
	clinique := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3,
		SubjectName: "patient", VisitName: "visite", ModalityName: "modalite"}
	if clinique.Describe() != "level 1 = patient, level 2 = visite, level 3 = modalite" {
		t.Fatalf("description = %q", clinique.Describe())
	}
	banque := PathLayout{SubjectLevel: 0, VisitLevel: 1, ModalityLevel: LevelAbsent,
		SubjectName: "client", VisitName: "mois"}
	if banque.Describe() != "level 0 = client, level 1 = mois" {
		t.Fatalf("description = %q", banque.Describe())
	}
	// Un nom tout en blancs retombe sur le defaut plutot que d'afficher du
	// vide : un libelle invisible est pire qu'un libelle generique.
	blanc := PathLayout{SubjectLevel: 0, VisitLevel: LevelAbsent, ModalityLevel: LevelAbsent,
		SubjectName: "   "}
	if blanc.Describe() != "level 0 = entity" {
		t.Fatalf("description = %q", blanc.Describe())
	}
	absente := PathLayout{SubjectLevel: LevelAbsent}
	if absente.Declared() {
		t.Fatal("une regle sans sujet n'est pas declaree")
	}
}

// Un fichier egare ne devient pas un patient.
//
// La regle est positionnelle par choix et ne juge pas a quoi ressemble un
// segment - c'est ce qui lui permet de lire SELENA-01-001, PREMYOM1000-0001
// et des numeros de client de banque avec un seul mecanisme. Le prix, c'est
// qu'elle appellerait n'importe quoi un sujet, et le 2026-10-08 elle l'a
// fait : declarer « niveau 1 = patient » sur SELENA a transforme
// SELENA/checksums/checksum_20260918.tsv en troisieme patient nomme
// « checksums », dont la visite etait le nom du fichier.
//
// La profondeur est le test qui garde la regle bete et juste quand meme.
func TestUnFichierEgareNeDevientPasUnSujet(t *testing.T) {
	layout := PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}

	sujet, _, _ := layout.Read("SELENA/checksums/checksum_20260918.tsv")
	if sujet != "" {
		t.Fatalf("un chemin sans niveau 3 ne decrit aucun sujet, obtenu %q", sujet)
	}

	// Et la vraie donnee passe toujours.
	sujet, visite, modalite := layout.Read("SELENA/SELENA-01-001/20260218/ANTERION/scan.dcm")
	if sujet != "SELENA-01-001" || visite != "20260218" || modalite != "ANTERION" {
		t.Fatalf("lecture = %q/%q/%q", sujet, visite, modalite)
	}
	// Y compris plus profond : l'export DICOM d'ANTERION descend a dix.
	sujet, _, modalite = layout.Read("SELENA/SELENA-01-001/20260218/ANTERION/DICOM/MC27/26070/105568/449313/00000656")
	if sujet != "SELENA-01-001" || modalite != "ANTERION" {
		t.Fatalf("un chemin profond doit rester lu : %q/%q", sujet, modalite)
	}
}

// Une regle qui ne nomme pas tous les niveaux n'exige pas ceux qu'elle tait.
//
// Une etude a une visite par patient n'a pas de niveau temporel, et exiger
// une profondeur pour un niveau absent rejetterait toute la donnee.
func TestUneRegleCourteNExigePasLesNiveauxQuElleTait(t *testing.T) {
	layout := PathLayout{SubjectLevel: 0, VisitLevel: LevelAbsent, ModalityLevel: 1}
	sujet, visite, modalite := layout.Read("PATIENT-7/IRM/scan.dcm")
	if sujet != "PATIENT-7" || modalite != "IRM" {
		t.Fatalf("lecture = %q/%q/%q", sujet, visite, modalite)
	}
	if visite != "" {
		t.Errorf("un niveau tu ne doit rien produire, obtenu %q", visite)
	}
}
