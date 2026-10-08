package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Le script de demarrage doit etre du shell valide, verifie par un shell.
//
// Il est assemble ligne par ligne en Go, avec des heredocs, des guillemets et
// des conditions imbriquees. Une erreur de syntaxe n'y est pas visible a la
// relecture et ne se manifeste qu'au demarrage d'un workspace, cote client,
// sous la forme d'un workspace qui "echoue". Le shell du conteneur est dash :
// "sh -n" est exactement le juge qu'il faut.
func TestLeScriptDeDemarrageEstDuShellValide(t *testing.T) {
	script := workspaceBootstrapScript(
		"vscode", "wks-1", "jeton", "Stef", "stef@example.org",
		false, "/home/noryx/.noryx-profile", "/mnt",
		[]workspaceAttachedRepo{{Name: "etude", URL: "https://example.org/etude.git", DefaultRef: "main"}},
		2, "models: []", true, 3,
		// Un extrait ecarte : sa raison est un echo, donc du shell a valider
		// comme le reste - et c'est exactement le genre de ligne qu'une
		// apostrophe dans un nom de dataset casserait.
		[]string{`selena: its dataset "HDS-For" isn't attached to this project`},
	)
	if !strings.Contains(script, "jupyter_server_config.py") {
		t.Fatal("le script ne contient plus l'ecriture qu'on cherche a proteger")
	}

	chemin := filepath.Join(t.TempDir(), "bootstrap.sh")
	if err := os.WriteFile(chemin, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	sortie, err := exec.Command("sh", "-n", chemin).CombinedOutput()
	if err != nil {
		t.Fatalf("le shell refuse le script : %v\n%s", err, sortie)
	}
}

// Et l'ecriture de la config ne tue plus le workspace.
//
// Trois volumes de profil sur EMSE portaient un dossier jupyter/config laisse
// par une epoque ou le workspace tournait en root. Sous "set -e", un fichier
// decoratif que le script ne pouvait pas reecrire coutait le workspace entier.
func TestUneConfigNonEcrivableNeTuePasLeWorkspace(t *testing.T) {
	script := workspaceBootstrapScript(
		"jupyter", "wks-2", "jeton", "Stef", "stef@example.org",
		false, "/home/noryx/.noryx-profile", "/mnt", nil, 0, "", false, 0, nil,
	)
	if !strings.Contains(script, "could not be written; continuing without it") {
		t.Fatal("l'echec d'ecriture de la config doit etre tolere et dit, pas fatal")
	}
	if !strings.Contains(script, "sudo chown -R noryx:noryx") {
		t.Fatal("le script doit tenter de reprendre un profil laisse a root")
	}
}
