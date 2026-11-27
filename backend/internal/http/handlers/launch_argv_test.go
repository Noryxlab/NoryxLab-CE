package handlers

import (
	"strings"
	"testing"
)

// Le cas qui a coute un apres-midi le 27/11/2026.
//
// La commande etait ecrite comme on l'ecrit dans un terminal. Decoupee en mots
// puis citee mot par mot, `&&` cesse d'etre un operateur : apt a recu la ligne
// entiere et a repondu « l'option --source n'est pas reconnue », qui ne nomme
// ni le `&&`, ni le decoupage, ni rien sur quoi la personne puisse agir.
func TestUneLigneShellSExecuteCommeUneLigneShell(t *testing.T) {
	mots := []string{
		"apt-get", "update", "-qq", "&&",
		"apt-get", "install", "-y", "-qq", "libusb-1.0-0", "&&",
		"python", "-u", "/mnt/cron/process_next.py",
		"--source", "/datasets/entree", "--processed", "/datasets/sortie",
	}
	rendu := launchWords(mots)
	if len(rendu) != 3 || rendu[0] != "sh" || rendu[1] != "-c" {
		t.Fatalf("attendu un passage par le shell, obtenu %v", rendu)
	}
	// Et la ligne doit etre complete : le `&&` y reste un operateur, pas un
	// argument cite.
	ligne := rendu[2]
	for _, attendu := range []string{"apt-get update", "&&", "--source", "process_next.py"} {
		if !strings.Contains(ligne, attendu) {
			t.Errorf("la ligne ne porte pas %q : %s", attendu, ligne)
		}
	}
}

// Ce que le quoting protege, et qu'on ne lache pas : un argument qui contient
// une espace reste un seul argument.
func TestUnArgumentAvecUneEspaceResteUnArgument(t *testing.T) {
	rendu := launchWords([]string{"python", "analyse.py", "--titre", "mon etude"})
	if len(rendu) != 4 {
		t.Fatalf("4 mots attendus, obtenu %d : %v", len(rendu), rendu)
	}
	if !strings.Contains(rendu[3], "mon etude") {
		t.Errorf("l'argument a ete casse : %v", rendu)
	}
	if rendu[0] == "sh" {
		t.Error("un argv ordinaire ne doit pas passer par un shell")
	}
}

// Un mot qui *contient* un operateur est un argument, pas un operateur. Une
// expression reguliere ou une URL ne doit pas faire basculer la commande dans
// un shell, ou son contenu serait relu.
func TestUnMotQuiContientUnOperateurResteUnArgument(t *testing.T) {
	for _, argument := range []string{"a&&b", "http://x/y;z", "'a|b'", "x>y"} {
		rendu := launchWords([]string{"grep", argument, "fichier.txt"})
		if rendu[0] == "sh" {
			t.Errorf("%q a fait basculer la commande dans un shell", argument)
		}
	}
}

// Le cas qui existait deja : un seul mot plein d'espaces est une ligne de
// commande, pas le nom d'un programme tres long.
func TestUnSeulMotPleinDEspacesPasseParLeShell(t *testing.T) {
	rendu := launchWords([]string{"python -u script.py --flag"})
	if len(rendu) != 3 || rendu[0] != "sh" || rendu[1] != "-c" {
		t.Fatalf("attendu sh -c, obtenu %v", rendu)
	}
}

// Les bords : rien a lancer ne lance rien, et les mots vides disparaissent
// avant d'arriver ici.
func TestLesBordsSontCalmes(t *testing.T) {
	if got := launchWords(nil); len(got) != 0 {
		t.Fatalf("nil a produit %v", got)
	}
	if got := commandWords([]string{" ", "python", "", "x.py"}); strings.Join(got, ",") != "python,x.py" {
		t.Fatalf("les mots vides survivent : %v", got)
	}
}
