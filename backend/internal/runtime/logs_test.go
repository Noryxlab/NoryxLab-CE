package runtime

import (
	"strings"
	"testing"
)

// Les journaux arrivaient bruts dans le navigateur, donc la couleur du
// terminal s'y lisait comme du texte : "[36mINFO[0m[0000] Retrieving image".
func TestLaMiseEnFormeDuTerminalDisparait(t *testing.T) {
	brut := "\x1b[36mINFO\x1b[0m[0000] Retrieving image manifest noryx-jupyter:0.1.1 \n"
	got := ReadableLogs(brut)
	if got != "INFO[0000] Retrieving image manifest noryx-jupyter:0.1.1 \n" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "[36m") || strings.Contains(got, "\x1b") {
		t.Fatal("ni le code ni l'echappement ne doivent rester")
	}
}

// Une barre de progression reecrit sa ligne : un terminal montre le dernier
// etat, une page montrait les cinquante.
func TestUneLigneReecriteGardeSonDernierEtat(t *testing.T) {
	brut := "Downloading pandas\r  10%\r  60%\r 100%\nInstalling\n"
	got := ReadableLogs(brut)
	if got != " 100%\nInstalling\n" {
		t.Fatalf("got %q", got)
	}
}

// Le reste est le mot du constructeur, et il ne se reecrit pas : c'est ce
// qu'on recopie dans un ticket.
func TestLeTexteDuConstructeurEstIntact(t *testing.T) {
	brut := "error building image: GET https://index.docker.io/v2/: UNAUTHORIZED\n"
	if got := ReadableLogs(brut); got != brut {
		t.Fatalf("got %q", got)
	}
	if got := ReadableLogs(""); got != "" {
		t.Fatalf("got %q", got)
	}
}
