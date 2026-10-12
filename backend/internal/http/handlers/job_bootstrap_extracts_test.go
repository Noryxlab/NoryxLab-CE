package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Un job prepare l'environnement avant de lancer la commande.
//
// Le script ne se construisait que si l'appelant ne nommait aucune commande -
// et l'ecran en nomme une : le formulaire envoie ce que la personne a tape
// dans `command`. Chaque job et chaque planification partis de l'UI tournaient
// donc comme entrypoint du conteneur, sans arbre d'extraits sous /extracts,
// sans depot dans /repos et sans requirements installes, le secret de
// bootstrap monte a cote et lu par personne.
func TestLeScriptDeJobMonteLesExtraitsAvantLaCommande(t *testing.T) {
	script := jobBootstrapScript(
		[]string{"python3", "analyse.py"},
		nil,
		extractMount{Names: []string{"anterion-3-sujets"}, Manifest: "ZmFrZQ=="},
	)
	for _, attendu := range []string{
		"/extracts",
		"/var/run/noryx/bootstrap/extracts.b64",
		"building extract links",
	} {
		if !strings.Contains(script, attendu) {
			t.Fatalf("le script doit monter les extraits, %q manque :\n%s", attendu, script)
		}
	}
	// Et dans cet ordre : le code lit dedans.
	if strings.Index(script, "extracts.b64") > strings.Index(script, "exec 'python3'") {
		t.Fatal("l'arbre des extraits doit etre construit avant la commande")
	}
}

// Et les mots de la commande restent des mots.
func TestLeScriptDeJobGardeLesFrontieresDeMots(t *testing.T) {
	script := jobBootstrapScript([]string{"python3", "-c", "print(1); print(2)"}, nil, extractMount{})
	if !strings.Contains(script, "exec 'python3' '-c' 'print(1); print(2)'") {
		t.Fatalf("chaque mot doit rester un mot :\n%s", script)
	}
	// Une ligne de commande arrivee en un seul mot passe par un shell, comme
	// pour une app.
	seule := jobBootstrapScript([]string{"python3 -m http.server 9000"}, nil, extractMount{})
	if !strings.Contains(seule, "exec sh -c 'python3 -m http.server 9000'") {
		t.Fatalf("une ligne en un mot doit passer par un shell :\n%s", seule)
	}
}

// Et le script produit reste du shell valide, juge par un shell.
func TestLeScriptDeJobEstDuShellValide(t *testing.T) {
	for _, argv := range [][]string{
		{"python3", "analyse.py"},
		{"python3 -m http.server 9000"},
		{"python3", "-c", "print(1); print(2)"},
		nil,
	} {
		script := jobBootstrapScript(argv, nil, extractMount{Names: []string{"x"}, Manifest: "ZmFrZQ=="})
		chemin := filepath.Join(t.TempDir(), "job.sh")
		if err := os.WriteFile(chemin, []byte(script), 0o600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("sh", "-n", chemin).CombinedOutput(); err != nil {
			t.Fatalf("argv %v produit un script invalide: %v\n%s", argv, err, out)
		}
	}
}
