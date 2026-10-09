package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Le secret "user:token" doit arriver au clone comme un couple, pas comme un
// mot de passe entier.
//
// La convention existait dans un seul des deux endroits qui en avaient besoin.
// La validation la decoupait et s'authentifiait correctement, donc « Tester »
// annoncait le depot joignable ; le clone envoyait la chaine entiere comme mot
// de passe sous un identifiant « oauth2 » code en dur, et echouait. Quelqu'un
// suivant le conseil que l'interface donne elle-meme - « pour Azure DevOps,
// stockez le secret sous la forme user:personal-access-token » - obtenait un
// depot qui valide et qui ne clone pas. C'est le pire des trois cas : l'ecran
// dit que tout va bien.
func TestLeCoupleIdentifiantMotDePasseEstDecoupePourLeClone(t *testing.T) {
	lignes := repositoryBootstrapLines(workspaceAttachedRepo{
		Name: "etude", URL: "https://dev.azure.com/essilor/_git/etude",
		AuthEnvName: "NORYX_SECRET_AZURE",
	}, "/mnt/etude")
	script := strings.Join(lignes, "\n")

	// Le decoupage se fait dans le script, parce que le secret arrive au pod
	// par une variable d'environnement et ne passe jamais par ce processus.
	if !strings.Contains(script, "user=${secret%%:*}") || !strings.Contains(script, "pass=${secret#*:}") {
		t.Errorf("le script doit decouper le secret :\n%s", script)
	}
	// Et plus aucun identifiant code en dur hors du cas « jeton seul ».
	if strings.Contains(script, "printf 'username=oauth2") {
		t.Error("l'identifiant ne doit plus etre fige dans la sortie du helper")
	}
	if !strings.Contains(script, "user=oauth2; pass=$secret") {
		t.Error("un jeton seul doit garder oauth2 comme identifiant")
	}
	// Chaque depot lit sa propre variable : deux depots attaches ne doivent
	// pas pouvoir se passer leurs identifiants par une variable exportee.
	if !strings.Contains(script, "secret=$NORYX_SECRET_AZURE") {
		t.Error("le script doit lire la variable de son propre depot")
	}
	if strings.Contains(script, "export ") {
		t.Error("pas d'export : deux depots partageraient la variable")
	}
}

// Ce que le shell en fait reellement, pour les deux formes de secret.
//
// Verifie en executant le fragment, parce que du shell assemble en Go ne se
// relit pas : c'est la substitution de parametres qui est en cause, et seul un
// shell dit ce qu'elle produit.
func TestLeShellProduitLeBonCoupleDansLesDeuxCas(t *testing.T) {
	for _, cas := range []struct{ secret, user, pass string }{
		{"monuser:monjeton", "monuser", "monjeton"},
		{"monjeton", "oauth2", "monjeton"},
		// Un identifiant vide est la forme canonique d'Azure DevOps, et elle
		// doit traverser telle quelle plutot que de retomber sur oauth2.
		{":monjeton", "", "monjeton"},
	} {
		fragment := `secret="$SECRET"
case "$secret" in
  *:*) user=${secret%%:*}; pass=${secret#*:} ;;
  *) user=oauth2; pass=$secret ;;
esac
printf '%s|%s' "$user" "$pass"`
		chemin := filepath.Join(t.TempDir(), "split.sh")
		if err := os.WriteFile(chemin, []byte(fragment), 0o600); err != nil {
			t.Fatal(err)
		}
		commande := exec.Command("sh", chemin)
		commande.Env = append(os.Environ(), "SECRET="+cas.secret)
		sortie, err := commande.Output()
		if err != nil {
			t.Fatalf("%q : %v", cas.secret, err)
		}
		attendu := cas.user + "|" + cas.pass
		if string(sortie) != attendu {
			t.Errorf("secret %q donne %q, attendu %q", cas.secret, sortie, attendu)
		}
	}
}

// Le script reste du shell valide avec un depot authentifie.
//
// Le test de syntaxe existant n'attachait qu'un depot sans identifiants,
// donc toute la branche qui ecrit les deux scripts d'aide n'etait jamais
// passee au juge.
func TestLeScriptAvecDepotAuthentifieEstDuShellValide(t *testing.T) {
	script := workspaceBootstrapScript(
		"vscode", "wks-1", "jeton", "Stef", "stef@example.org",
		false, "/home/noryx/.noryx-profile", "/mnt",
		[]workspaceAttachedRepo{{
			Name: "etude", URL: "https://dev.azure.com/essilor/_git/etude",
			DefaultRef: "main", AuthEnvName: "NORYX_SECRET_AZURE",
		}},
		2, "models: []", true, 3, nil,
	)
	chemin := filepath.Join(t.TempDir(), "bootstrap.sh")
	if err := os.WriteFile(chemin, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if sortie, err := exec.Command("sh", "-n", chemin).CombinedOutput(); err != nil {
		t.Fatalf("le shell refuse le script : %v\n%s", err, sortie)
	}
}
