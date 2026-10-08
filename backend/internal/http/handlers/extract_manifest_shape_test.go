package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

// Le manifeste ecrit autant de colonnes que les scripts en lisent.
//
// C'est le defaut qui a coute un arbre d'extraits vide : deux scripts shell
// lisent ce fichier, chacun avait sa propre liste de variables, et quand les
// trois niveaux ont fusionne en un repertoire deja ordonne, seule une des deux
// a ete mise a jour. L'autre glissait de deux colonnes - "$path" sortait vide,
// la source designait un repertoire, et chaque fichier etait annonce manquant
// dans un journal que personne ne lisait.
//
// Le test ne verifie pas une formulation : il verifie que les deux lecteurs
// nomment exactement les colonnes que l'encodeur produit.
func TestLeManifesteEtSesLecteursSAccordent(t *testing.T) {
	entries := []extractMountEntry{{
		ExtractName: "selena",
		DatasetDir:  "hds-essilor",
		Dir:         "SELENA-01-001/20260218/ANTERION",
		Path:        "SELENA-01-001/20260218/ANTERION/DICOM/00000121",
		Leaf:        "DICOM/00000121",
	}}
	encoded, ok := encodeExtractManifest(entries)
	if !ok || encoded == "" {
		t.Fatal("le manifeste ne s'encode pas")
	}

	colonnes := len(extractManifestFields)
	ligne := extractManifestReadLine()
	for _, nom := range extractManifestFields {
		if !strings.Contains(ligne, " "+nom) {
			t.Fatalf("la ligne de lecture %q ne nomme pas la colonne %q", ligne, nom)
		}
	}
	// Autant de variables que de colonnes, pas une de plus : une variable en
	// trop est precisement ce qui restait vide en silence.
	if lues := len(strings.Fields(strings.SplitN(ligne, "read -r ", 2)[1])); lues != colonnes {
		t.Fatalf("la ligne de lecture prend %d variables pour %d colonnes", lues, colonnes)
	}

	// Et les deux scripts passent bien par cette ligne, plutot que de porter
	// leur propre liste.
	bootstrap := strings.Join(extractBootstrapLines("/mnt", true, 0, nil), "\n")
	filler := extractFillerScript("/cache", "wks-test")
	for nom, script := range map[string]string{"bootstrap": bootstrap, "filler": filler} {
		if !strings.Contains(script, ligne) {
			t.Fatalf("le script %s ne derive pas sa lecture du manifeste", nom)
		}
		// Les anciens noms de niveaux ne doivent plus apparaitre : leur retour
		// signifierait qu'un troisieme lecteur a recommence.
		for _, ancien := range []string{"read -r extract dataset subject", "$subject\"/\"$visit", "$niveaux"} {
			if strings.Contains(script, ancien) {
				t.Fatalf("le script %s porte encore %q", nom, ancien)
			}
		}
	}
}

// Le repertoire du manifeste est celui que l'extrait a demande, pas un ordre
// fixe : c'est la seule chose qui rend la disposition declarable.
func TestLeManifestePorteLeRepertoireOrdonne(t *testing.T) {
	entries := []extractMountEntry{{
		ExtractName: "selena",
		DatasetDir:  "hds",
		Dir:         "ANTERION/SELENA-01-001/20260218", // modalite d'abord
		Path:        "a/b/c",
		Leaf:        "c",
	}}
	encoded, ok := encodeExtractManifest(entries)
	if !ok {
		t.Fatal("encodage refuse")
	}
	decoded := decodeExtractManifestForTest(t, encoded)
	if !strings.Contains(decoded, "ANTERION/SELENA-01-001/20260218") {
		t.Fatalf("le repertoire ordonne n'est pas dans le manifeste : %q", decoded)
	}
	// Et il est dans la colonne que les scripts lisent comme "dir".
	champs := strings.Split(strings.TrimSpace(decoded), "\t")
	position := -1
	for i, nom := range extractManifestFields {
		if nom == "dir" {
			position = i
		}
	}
	if position < 0 || position >= len(champs) || champs[position] != "ANTERION/SELENA-01-001/20260218" {
		t.Fatalf("colonne dir = %d, champs = %q", position, champs)
	}
}

func decodeExtractManifestForTest(t *testing.T, encoded string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("lecture: %v", err)
	}
	return string(out)
}
