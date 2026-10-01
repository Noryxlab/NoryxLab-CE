package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Une ligne de commande arrivee en un seul mot doit demarrer.
//
// Le contrat est un argv en mots, et le formulaire le respecte. Qui appelle
// l'API a la main ecrit ce qu'il aurait tape : le controle quotidien de la
// plateforme envoyait {"args":["python3 -m http.server 9000 ..."]}, ce qui
// produisait `exec 'python3 -m http.server ...'` - un seul mot, donc un
// programme portant ce nom, introuvable. Code 127 une seconde apres le
// demarrage, et une app "en echec" sans autre explication.
func TestUneLigneDeCommandeEnUnMotPasseParUnShell(t *testing.T) {
	script := appBootstrapScript(9000, []string{"python3 -m http.server 9000 --directory /tmp"}, nil)
	if !strings.Contains(script, "exec sh -c 'python3 -m http.server 9000 --directory /tmp'") {
		t.Fatalf("la ligne de commande doit passer par un shell :\n%s", script)
	}
}

// Et un argv en plusieurs mots garde ses frontieres de mots.
//
// C'est la propriete inverse et elle compte autant : une app declaree
// {"command":["/bin/sh","-lc"],"args":["FOO=1 run.sh"]} doit donner
// `exec '/bin/sh' '-lc' 'FOO=1 run.sh'`, ou sh recoit bien la chaine entiere
// comme son programme - pas `exec /bin/sh -lc FOO=1 run.sh`, ou il n'en prend
// que le premier mot, execute l'affectation et sort 0 en annoncant un succes.
func TestUnArgvEnPlusieursMotsGardeSesMots(t *testing.T) {
	script := appBootstrapScript(9000, []string{"/bin/sh", "-lc", "FOO=1 run.sh"}, nil)
	if !strings.Contains(script, "exec '/bin/sh' '-lc' 'FOO=1 run.sh'") {
		t.Fatalf("chaque mot d'un argv doit rester un mot :\n%s", script)
	}
	if strings.Contains(script, "exec sh -c") {
		t.Fatal("un argv en plusieurs mots ne doit pas etre repasse a un shell")
	}
}

// Et le script produit reste du shell valide, juge par un shell.
func TestLeScriptDAppEstDuShellValide(t *testing.T) {
	for _, argv := range [][]string{
		{"python3 -m http.server 9000 --directory /tmp"},
		{"/bin/sh", "-lc", "FOO=1 run.sh"},
		nil,
	} {
		script := appBootstrapScript(9000, argv, nil)
		chemin := filepath.Join(t.TempDir(), "app.sh")
		if err := os.WriteFile(chemin, []byte(script), 0o600); err != nil {
			t.Fatal(err)
		}
		if sortie, err := exec.Command("sh", "-n", chemin).CombinedOutput(); err != nil {
			t.Fatalf("le shell refuse le script pour %v : %v\n%s", argv, err, sortie)
		}
	}
}
