package handlers

import (
	"strings"
	"testing"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
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

// Une disposition peut nommer un ou deux niveaux, et c'est la collision qui
// protege - pas le compte de niveaux.
//
// Les trois etaient exiges, donc l'arbre portait toujours un repertoire de
// visite meme sur une etude ou chaque patient n'en a qu'une : un niveau qui
// ne sert qu'a etre traverse. Le refus s'est deplace la ou il peut etre juste,
// contre les fichiers reellement selectionnes.
func TestUneDispositionPeutNommerMoinsDeTroisNiveaux(t *testing.T) {
	for _, essai := range [][]string{
		{"subject"},
		{"subject", "modality"},
		{"modality"},
	} {
		if _, probleme := extractdomain.NormaliseLayout(essai); probleme != "" {
			t.Fatalf("%v refuse : %s", essai, probleme)
		}
	}
}

// Ce qui reste refuse : un niveau repete, et un mot qui n'est pas un niveau.
func TestUneDispositionMalFormeeEstRefusee(t *testing.T) {
	for _, essai := range [][]string{
		{"subject", "subject", "modality"},
		{"patient", "visit", "modality"},
		{"subject", "visite"},
	} {
		if _, probleme := extractdomain.NormaliseLayout(essai); probleme == "" {
			t.Fatalf("%v aurait du etre refuse", essai)
		}
	}
}

// Omettre un niveau est refuse quand ca ecrase, accepte quand ca n'ecrase pas.
//
// C'est la distinction entiere. Sur SELENA chaque patient n'a qu'une visite,
// donc « patient / modalite » ne perd rien ; sur une etude a deux visites, la
// meme disposition ferait disparaitre la seconde en silence - et un fichier
// absent dont personne n'est prevenu est la pire facon de se tromper sur des
// donnees.
func TestUneDispositionQuiEcraseEstRefusee(t *testing.T) {
	uneSeuleVisite := []ontologydomain.Object{
		{Path: "SELENA/P1/20260218/ANTERION/DICOM/1", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
		{Path: "SELENA/P2/20260422/ANTERION/DICOM/1", SubjectID: "P2", Visit: "20260422", Modality: "ANTERION"},
	}
	if _, collide := layoutCollision([]string{"subject", "modality"}, uneSeuleVisite); collide {
		t.Fatal("un patient par visite ne peut pas entrer en collision")
	}

	deuxVisites := []ontologydomain.Object{
		{Path: "ETUDE/P1/20260218/ANTERION/DICOM/1", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
		{Path: "ETUDE/P1/20260422/ANTERION/DICOM/1", SubjectID: "P1", Visit: "20260422", Modality: "ANTERION"},
	}
	message, collide := layoutCollision([]string{"subject", "modality"}, deuxVisites)
	if !collide {
		t.Fatal("deux visites rangees sans niveau de visite doivent etre refusees")
	}
	// Le message doit etre actionnable : nommer le niveau a remettre.
	if !strings.Contains(message, "visit") {
		t.Errorf("le refus doit dire quoi ajouter : %s", message)
	}
	// Et nommer les deux fichiers, pour que la personne reconnaisse sa donnee.
	if !strings.Contains(message, "20260218") || !strings.Contains(message, "20260422") {
		t.Errorf("le refus doit nommer les deux chemins : %s", message)
	}
}

// Les trois niveaux au complet ne peuvent pas entrer en collision : deux
// fichiers d'un meme sujet, visite et modalite gardent leur chemin propre en
// dessous.
func TestLesTroisNiveauxNeCollisionnentPas(t *testing.T) {
	membres := []ontologydomain.Object{
		{Path: "ETUDE/P1/20260218/ANTERION/DICOM/MC27/1", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
		{Path: "ETUDE/P1/20260218/ANTERION/DICOM/MC28/1", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
	}
	if _, collide := layoutCollision(extractdomain.DefaultLayout(), membres); collide {
		t.Fatal("la disposition complete ne doit jamais ecraser")
	}
}

// Les six ordres sont acceptes - l'ecran n'en proposait que trois.
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
	lignes := strings.Join(extractBootstrapLines("/mnt", true, 0, nil), "\n")
	if strings.Contains(lignes, `"$subject"/"$visit"/"$modality"`) {
		t.Fatalf("le script compose encore les niveaux en dur :\n%s", lignes)
	}
	// Le nom de la variable vient de extractManifestFields, pour que renommer
	// une colonne ne laisse pas ce test valider l'ancien monde.
	attendu := `"$dir"`
	for _, nom := range extractManifestFields {
		if nom == "dir" {
			attendu = `"$` + nom + `"`
		}
	}
	if !strings.Contains(lignes, attendu) {
		t.Fatalf("le script doit lire le repertoire deja ordonne (%s) :\n%s", attendu, lignes)
	}
}

// La verification compose exactement ce que compose le montage.
//
// C'est tout l'interet : un controle qui construit un arbre legerement
// different repond sur un arbre que personne ne construit. Deux valeurs
// distinctes qui donnent le meme nom de repertoire apres assainissement
// passeraient ici et se percuteraient la-bas - l'echec meme qu'on veut
// eviter, entre par la porte de derriere.
func TestLaVerificationAssainitCommeLeMontage(t *testing.T) {
	// L'assainissement met en minuscules, donc deux sujets qui ne different
	// que par la casse partagent un repertoire. Ca arrive sur un bucket
	// rempli a la main, et la disposition complete ne protege pas de ca.
	if sanitizeWorkspacePathName("P1") != sanitizeWorkspacePathName("p1") {
		t.Skip("l'assainissement ne confond plus la casse")
	}
	membres := []ontologydomain.Object{
		{Path: "ETUDE/P1/20260218/ANTERION/1", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
		{Path: "ETUDE/p1/20260218/ANTERION/1", SubjectID: "p1", Visit: "20260218", Modality: "ANTERION"},
	}
	if _, collide := layoutCollision(extractdomain.DefaultLayout(), membres); !collide {
		t.Fatal("deux sujets qui s'assainissent pareil doivent etre vus avant le montage")
	}
}

// Les cles de dossier sont ecartees avant le controle, comme au montage.
//
// Les monter cachait l'etude - un extrait de 4 019 fichiers en avait monte
// 186 le 2026-10-01 - donc le montage les jette. Refuser une disposition
// parce que deux d'entre elles se percutent serait refuser sur des fichiers
// qui n'allaient jamais etre lies.
func TestLesClesDeDossierNeFontPasEchouerLaVerification(t *testing.T) {
	membres := []ontologydomain.Object{
		{Path: "ETUDE/P1/20260218/ANTERION", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
		{Path: "ETUDE/P1/20260218/ANTERION/DICOM/1", SubjectID: "P1", Visit: "20260218", Modality: "ANTERION"},
		{Path: "ETUDE/P2/20260422/ANTERION", SubjectID: "P2", Visit: "20260422", Modality: "ANTERION"},
		{Path: "ETUDE/P2/20260422/ANTERION/DICOM/1", SubjectID: "P2", Visit: "20260422", Modality: "ANTERION"},
	}
	if _, collide := layoutCollision([]string{"subject", "modality"}, membres); collide {
		t.Fatal("les cles de dossier ne doivent pas provoquer de refus")
	}
}
