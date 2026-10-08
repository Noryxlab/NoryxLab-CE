package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The extract mounts as links over the dataset that is already there. If this
// ever became a copy, a 400 GB study would be duplicated per workspace - and,
// worse, the platform would be writing a second copy of regulated data it was
// only ever meant to read.
func TestExtractBootstrapLinksAndNeverCopies(t *testing.T) {
	script := strings.Join(extractBootstrapLines("/mnt", true, 0, nil), "\n")
	if !strings.Contains(script, "ln -sfn") {
		t.Fatal("the extract tree must be built from symlinks")
	}
	for _, forbidden := range []string{"cp ", "rsync", "mc cp", "aws s3 cp"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("extract bootstrap copies data (%q); it must only link", forbidden)
		}
	}
}

// An extract too large for one manifest is said out loud. A partial tree would
// look like a complete study and quietly change someone's n.
func TestOversizedExtractIsRefusedOutLoud(t *testing.T) {
	script := strings.Join(extractBootstrapLines("/mnt", false, 41234, nil), "\n")
	if !strings.Contains(script, "41234") || !strings.Contains(script, "not mounted") {
		t.Fatalf("an unmountable extract must say so; got: %s", script)
	}
	if strings.Contains(script, "ln -sfn") {
		t.Fatal("a refused extract must not build a partial tree")
	}
}

// A tab or a newline in a key would split a record and file an object under the
// wrong subject. Such a key is dropped, never repaired into something plausible.
func TestExtractManifestDropsKeysThatWouldSplitARecord(t *testing.T) {
	encoded, ok := encodeExtractManifest([]extractMountEntry{
		{ExtractName: "c", DatasetDir: "d", Dir: "S1/v1/m", Path: "good/file.csv"},
		{ExtractName: "c", DatasetDir: "d", Dir: "S1/v1/m", Path: "bad\tfile.csv"},
	})
	if !ok || encoded == "" {
		t.Fatal("a small manifest must encode")
	}
}

// Un extrait qu'on ne peut pas monter le dit dans le demarrage, pas dans un
// log serveur que personne ne lit.
//
// C'etait un log.Printf : on attachait un extrait a un projet, on ouvrait un
// workspace, /extracts etait vide, et rien nulle part ne disait que le
// dataset d'origine n'etait pas monte la. Un repertoire vide ressemble a une
// panne de la plateforme alors que c'est une case a cocher qui manque.
func TestUnExtraitEcarteLeDitAuDemarrage(t *testing.T) {
	raison := `selena: its dataset "HDS-For" is not attached to this project`
	lignes := strings.Join(extractBootstrapLines("/mnt", false, 0, []string{raison}), "\n")
	if !strings.Contains(lignes, "HDS-For") {
		t.Fatalf("la raison doit apparaitre dans le demarrage :\n%s", lignes)
	}
	if !strings.Contains(lignes, "extract not mounted") {
		t.Error("la ligne doit dire qu'un extrait n'a pas ete monte")
	}
}

// Et la raison est du shell valide, meme quand un nom porte une apostrophe.
//
// Elle part dans un echo, donc un nom de dataset mal choisi ferait echouer
// tout le script de demarrage - le workspace entier, pour un message.
func TestUneRaisonAvecApostropheNeCassePasLeDemarrage(t *testing.T) {
	lignes := strings.Join(
		extractBootstrapLines("/mnt", false, 0, []string{`l'etude: its dataset "d'Essilor" is not attached`}),
		"\n",
	)
	chemin := filepath.Join(t.TempDir(), "extraits.sh")
	if err := os.WriteFile(chemin, []byte("#!/bin/sh\n"+lignes+"\n"), 0o600); err != nil {
		t.Fatalf("ecriture : %v", err)
	}
	if sortie, err := exec.Command("sh", "-n", chemin).CombinedOutput(); err != nil {
		t.Fatalf("sh -n refuse le script : %v\n%s", err, sortie)
	}
}
