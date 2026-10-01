package handlers

import (
	"strings"
	"testing"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
)

// La disposition est la seconde moitie de ce qu'est un extrait.
//
// Une selection dit QUELS fichiers ; une disposition dit comment ils sont
// ranges pour travailler. La meme selection organisee par sujet repond a "que
// possede ce patient", et par modalite a "montre-moi tous mes scans de
// cornee" - deux questions, un seul ensemble de fichiers. L'arbre etait
// construit dans un ordre fixe, donc seule la premiere etait exprimable.
func TestLaDispositionRangeLesMemesFichiersAutrement(t *testing.T) {
	parSujet := extractdomain.DirectoryFor(
		[]string{extractdomain.LevelSubject, extractdomain.LevelVisit, extractdomain.LevelModality},
		"s1", "20260218", "anterion")
	if strings.Join(parSujet, "/") != "s1/20260218/anterion" {
		t.Fatalf("par sujet = %v", parSujet)
	}
	parModalite := extractdomain.DirectoryFor(
		[]string{extractdomain.LevelModality, extractdomain.LevelSubject, extractdomain.LevelVisit},
		"s1", "20260218", "anterion")
	if strings.Join(parModalite, "/") != "anterion/s1/20260218" {
		t.Fatalf("par modalite = %v", parModalite)
	}
}

// Vide veut dire la disposition par defaut, pas une erreur.
//
// Un extrait declare avant que la disposition existe n'en porte aucune, et son
// arbre ne doit pas bouger sous lui.
func TestUneDispositionVideEstCelleParDefaut(t *testing.T) {
	layout, probleme := extractdomain.NormaliseLayout(nil)
	if probleme != "" {
		t.Fatalf("vide doit etre accepte : %s", probleme)
	}
	if strings.Join(layout, "/") != "subject/visit/modality" {
		t.Fatalf("defaut = %v, attendu sujet d'abord", layout)
	}
	// Et DirectoryFor suit le defaut sans qu'on le lui passe.
	if strings.Join(extractdomain.DirectoryFor(nil, "s1", "v1", "m1"), "/") != "s1/v1/m1" {
		t.Fatal("une disposition absente doit ranger par sujet d'abord")
	}
}

// Une disposition partielle est refusee, et c'est le point qui protege.
//
// Omettre un niveau mettrait dans un meme repertoire des fichiers de visites
// differentes : ceux qui partagent un nom s'ecraseraient, et l'etude perdrait
// des lignes en silence.
func TestUneDispositionPartielleEstRefusee(t *testing.T) {
	for _, essai := range [][]string{
		{"subject"},
		{"subject", "modality"},
		{"subject", "subject", "modality"},
		{"patient", "visit", "modality"},
	} {
		if _, probleme := extractdomain.NormaliseLayout(essai); probleme == "" {
			t.Fatalf("%v aurait du etre refuse", essai)
		}
	}
}

// Les trois niveaux, dans n'importe quel ordre, sont acceptes.
func TestToutesLesPermutationsSontAcceptees(t *testing.T) {
	ordres := [][]string{
		{"subject", "visit", "modality"},
		{"modality", "subject", "visit"},
		{"visit", "modality", "subject"},
		{"MODALITY", " Subject ", "visit"},
	}
	for _, ordre := range ordres {
		if _, probleme := extractdomain.NormaliseLayout(ordre); probleme != "" {
			t.Fatalf("%v refuse : %s", ordre, probleme)
		}
	}
}

// Le script de montage ne compose plus les niveaux lui-meme.
//
// C'est ce qui rendait l'ordre impossible a changer : le shell joignait sujet,
// visite et modalite dans un ordre ecrit en dur. Le repertoire arrive
// maintenant deja ordonne.
func TestLeScriptNeComposePlusLesNiveaux(t *testing.T) {
	lignes := strings.Join(extractBootstrapLines("/mnt", true, 0), "\n")
	if strings.Contains(lignes, `"$subject"/"$visit"/"$modality"`) {
		t.Fatalf("le script compose encore les niveaux en dur :\n%s", lignes)
	}
	if !strings.Contains(lignes, `"$niveaux"`) {
		t.Fatalf("le script doit lire le repertoire deja ordonne :\n%s", lignes)
	}
}
