package handlers

import (
	"strings"
	"testing"
)

// A scan of a dataset laid out differently used to report "24,179 objects, 0
// subjects" and drop every path in silence. That reads as a broken feature
// rather than as "this layout is not the one the profile knows", which is what
// it is.
func TestAPathTheProfileCannotReadIsDescribedRatherThanDropped(t *testing.T) {
	// The convention the profile knows.
	shape := describePathShape("PREMYOM1000-0001/20260115/ANTERION/scan.dcm")
	if !strings.Contains(shape, "subject") || !strings.Contains(shape, "date") {
		t.Errorf("the known layout should be recognised: %s", shape)
	}

	// A deeper one, of the kind HDS-For holds: ten levels, the subject buried.
	deep := describePathShape("site-a/study/PREMYOM1000-0002/exam/20260115/device/series/1/2/image.dcm")
	if !strings.HasPrefix(deep, "10 levels") {
		t.Errorf("the depth should lead: %s", deep)
	}
	if !strings.Contains(deep, "DICOM") {
		t.Errorf("the format should be named: %s", deep)
	}

	// And nothing identifying leaves the scan: these are health-context
	// metadata, and a diagnosis does not need the identifiers.
	for _, identifier := range []string{"PREMYOM1000-0002", "site-a", "image.dcm", "20260115"} {
		if strings.Contains(deep, identifier) {
			t.Errorf("the shape leaks %q: %s", identifier, deep)
		}
	}
}

// Every shape is reported, commonest first.
//
// It used to keep the eight commonest and say nothing about the rest, so the
// panel read as an inventory while hiding a tail: on SELENA the eight shown
// accounted for 3,979 objects out of 3,994, and the fifteen others appeared
// nowhere. A list that looks exhaustive and is not is worse than a short one,
// because nobody thinks to ask what is missing. How many to show at once is
// the screen's decision, and it can only offer "show the rest" if it is given
// the rest.
func TestEveryShapeIsReportedCommonestFirst(t *testing.T) {
	layouts := map[string]int{}
	for index := 0; index < 20; index++ {
		layouts[strings.Repeat("x", index+1)] = index + 1
	}
	described := describeLayouts(layouts)
	if len(described) != 20 {
		t.Fatalf("expected every shape, got %d", len(described))
	}
	if !strings.Contains(described[0], "(20)") {
		t.Errorf("the commonest should lead: %s", described[0])
	}
	if !strings.Contains(described[len(described)-1], "(1)") {
		t.Errorf("the rarest should close the list: %s", described[len(described)-1])
	}
}

func TestTheCommonestUnreadableLayoutComesFirst(t *testing.T) {
	described := describeLayouts(map[string]int{
		"4 levels · text/text/text/file":                                 12,
		"10 levels · text/text/text/text/text/text/text/text/text/DICOM": 19022,
		"2 levels · text/CSV":                                            300,
	})
	if len(described) != 3 {
		t.Fatalf("expected three shapes, got %d", len(described))
	}
	if !strings.HasPrefix(described[0], "10 levels") || !strings.Contains(described[0], "(19022)") {
		t.Errorf("the commonest shape should lead, with its count: %s", described[0])
	}
	if describeLayouts(nil) != nil {
		t.Error("no unreadable path means nothing to describe")
	}
}

// Les formes qui ont marche, et celles qui ont echoue, sont deux choses.
//
// LayoutSamples repond a "pourquoi le scan n a rien lu", donc il ne portait que
// les formes en echec. C est la mauvaise moitie pour qui veut decrire un jeu de
// donnees : sur SELENA le scan a reconnu 4 023 objets sur 4 026, et les seules
// formes enregistrees etaient les trois qu il avait manquees. Un assistant a
// qui on montre celles-la n apprend rien, et il l a dit - a juste titre,
// puisqu on ne lui avait rien montre des donnees.
func TestLesFormesReconnuesSontDecritesAPart(t *testing.T) {
	reconnus := map[string]int{
		"4 levels · text/subject/date/DICOM":      4000,
		"5 levels · text/subject/date/text/DICOM": 23,
	}
	rates := map[string]int{"3 levels · text/text/TSV": 3}

	decrits := describeLayouts(reconnus)
	if len(decrits) == 0 {
		t.Fatal("aucune forme reconnue decrite")
	}
	// La plus frequente doit figurer : c est elle qui dit a quoi ressemble le
	// jeu de donnees.
	trouve := false
	for _, d := range decrits {
		if strings.Contains(d, "subject") {
			trouve = true
		}
	}
	if !trouve {
		t.Errorf("la forme majoritaire n apparait pas : %v", decrits)
	}
	// Et les deux ensembles ne se melangent pas.
	for _, d := range describeLayouts(rates) {
		for _, r := range decrits {
			if d == r {
				t.Errorf("une forme ratee apparait parmi les reconnues : %q", d)
			}
		}
	}
}

// Une table de mesure est un axe, pas une liste de noms de fichiers.
//
// Le prefixe sujet etait conserve, donc la meme table apparaissait une fois
// par sujet : PREMYOM1000-0001_Cornea_Basics, PREMYOM1000-0002_Cornea_Basics,
// trente fois. Rien ne se comptait a travers les sujets et rien ne se
// demandait. Depouille, l'ANTERION de PREMYOM1000 a sept tables, chacune
// chez les trente sujets.
func TestUneTableDeMesureEstPartageeEntreLesSujets(t *testing.T) {
	premier := inferMeasurementTable("PREMYOM1000/PREMYOM1000-0001/20250430/ANTERION/PREMYOM1000-0001_Cornea_Basics.csv", "CSV", "PREMYOM1000-0001")
	second := inferMeasurementTable("PREMYOM1000/PREMYOM1000-0002/20250501/ANTERION/PREMYOM1000-0002_Cornea_Basics.csv", "CSV", "PREMYOM1000-0002")
	if premier != "Cornea_Basics" {
		t.Fatalf("table = %q, attendu Cornea_Basics", premier)
	}
	if premier != second {
		t.Errorf("deux sujets doivent partager la table : %q et %q", premier, second)
	}
}

// Ce qui n'est pas une table reste sans nom plutot que devine.
func TestCeQuiNEstPasUneTableResteSansNom(t *testing.T) {
	for _, essai := range []struct{ chemin, format, sujet string }{
		// Un export par acquisition : un fichier chacun, et ils enterreraient
		// les sept vraies tables.
		{"e/p1/20250507/ANTERION/1_ANTERION_COR_2025-05-07_111013_OD.csv", "CSV", "p1"},
		// Le nom n'est que l'identifiant du sujet.
		{"e/PREMYOM1000-0001/20250430/IRM/PREMYOM1000-0001.csv", "CSV", "autre"},
		// Et une image n'est pas une table.
		{"e/p1/20250430/ANTERION/DICOM/MC27/00000001", "", "p1"},
	} {
		if nom := inferMeasurementTable(essai.chemin, essai.format, essai.sujet); nom != "" && essai.format == "" {
			t.Errorf("%q ne devrait nommer aucune table, obtenu %q", essai.chemin, nom)
		}
	}
	// Le cas qui compte vraiment : un fichier dont le nom est exactement
	// l'identifiant du sujet, une fois le prefixe retire.
	if nom := inferMeasurementTable("e/p1/20250430/IRM/PREMYOM1000-0001.csv", "CSV", "autre"); nom != "" {
		t.Errorf("un identifiant de sujet ne nomme pas une table, obtenu %q", nom)
	}
}
