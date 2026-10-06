package handlers

import (
	"os/exec"
	"strings"
	"testing"
)

// Le fichier d'entree se lit dans la commande de lancement, sous les formes
// que les gens ecrivent vraiment.
func TestLeFichierDEntreeEstTrouveDansLaCommande(t *testing.T) {
	cas := []struct {
		nom    string
		argv   []string
		attend string
	}{
		{"streamlit", []string{"streamlit", "run", "app.py", "--server.port", "8501"}, "/mnt/app.py"},
		{"python3", []string{"python3", "main.py"}, "/mnt/main.py"},
		{"module", []string{"python", "-m", "streamlit", "run", "tableau.py"}, "/mnt/tableau.py"},
		{"chemin relatif", []string{"streamlit", "run", "./sous/app.py"}, "/mnt/sous/app.py"},
		{"chemin absolu", []string{"python3", "/repos/outil/main.py"}, "/repos/outil/main.py"},
		// Un seul mot qui porte des espaces est une ligne de commande : le
		// constructeur de lancement traite ce cas, celui-ci aussi.
		{"ligne de commande", []string{"streamlit run app.py --server.port 8501"}, "/mnt/app.py"},
		{"rien de python", []string{"/bin/sh", "-lc", "./run.sh"}, ""},
		{"serveur statique", nil, ""},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := pythonEntryFile(c.argv); got != c.attend {
				t.Fatalf("pythonEntryFile = %q, attendu %q", got, c.attend)
			}
		})
	}
}

// Sans fichier Python, rien n'est ajoute : il n'y a pas d'imports a lire, et
// une ligne rassurante sur un fichier qu'on n'a pas ouvert serait pire que le
// silence.
func TestSansFichierPythonAucuneVerification(t *testing.T) {
	if lignes := appDependencyCheckLines([]string{"/bin/sh", "-lc", "./run.sh"}); lignes != nil {
		t.Fatalf("des lignes ont ete ajoutees sans fichier Python : %v", lignes)
	}
	script := appBootstrapScript(9000, nil, nil, extractMount{})
	if strings.Contains(script, "noryx-check-deps") {
		t.Fatal("le serveur statique embarque une verification de dependances")
	}
}

// La verification arrive apres l'installation et avant le lancement : sinon
// "importable" ne veut pas dire ce qu'il voudra dire au demarrage, ou
// l'avertissement arrive apres la trace qu'il explique.
func TestLaVerificationEstEntreLInstallationEtLeLancement(t *testing.T) {
	script := appBootstrapScript(8501, []string{"streamlit", "run", "app.py"}, nil, extractMount{})
	install := strings.Index(script, "requirements installation completed")
	// Les chemins passent par shellQuote, donc entre apostrophes.
	check := strings.Index(script, "noryx-check-deps.py '/mnt/app.py'")
	// Chaque mot du lancement est quote lui aussi.
	lancement := strings.Index(script, "exec 'streamlit'")
	if install < 0 || check < 0 || lancement < 0 {
		t.Fatalf("une etape manque : install=%d check=%d lancement=%d", install, check, lancement)
	}
	if !(install < check && check < lancement) {
		t.Fatalf("mauvais ordre : install=%d check=%d lancement=%d", install, check, lancement)
	}
}

// Elle ne fait jamais echouer le lancement. Une application qui marche a
// moitie vaut mieux pour son proprietaire qu'un refus, et une verification qui
// peut coucher une application est une verification qu'on fera retirer.
func TestLaVerificationNePeutPasCoucherLApplication(t *testing.T) {
	script := appBootstrapScript(8501, []string{"streamlit", "run", "app.py"}, nil, extractMount{})
	ligne := ""
	for _, l := range strings.Split(script, "\n") {
		if strings.Contains(l, "noryx-check-deps.py '/mnt/app.py'") {
			ligne = l
		}
	}
	if ligne == "" {
		t.Fatal("la ligne d'execution est introuvable")
	}
	if !strings.HasSuffix(strings.TrimSpace(ligne), "|| true") {
		t.Fatalf("la verification peut faire echouer le script (set -e) : %q", ligne)
	}
}

// Le script engendre reste du shell valide.
//
// Le programme Python est injecte par heredoc et porte des apostrophes, des
// guillemets, des dollars et des accents graves - tout ce qui a un sens pour un
// shell et aucun pour Python. Une seule apostrophe mal placee dans un echo
// entre apostrophes casse le demarrage de toutes les applications, et le seul
// juge fiable est un shell.
func TestLeScriptEngendreEstDuShellValide(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("pas de shell pour juger")
	}
	for _, argv := range [][]string{
		{"streamlit", "run", "weather_app_tallinn.py", "--server.port", "8501"},
		{"python3", "main.py"},
		{"streamlit run app.py --server.port 8501"},
		nil,
	} {
		script := appBootstrapScript(8501, argv, nil, extractMount{})
		commande := exec.Command("sh", "-n")
		commande.Stdin = strings.NewReader(script)
		if sortie, err := commande.CombinedOutput(); err != nil {
			t.Fatalf("shell invalide pour %v : %v\n%s", argv, err, sortie)
		}
	}
}

// Le programme Python est du Python valide, juge par Python.
func TestLeProgrammeEstDuPythonValide(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("pas de python3 pour juger")
	}
	commande := exec.Command(python, "-c",
		"import ast,sys; ast.parse(sys.stdin.read())")
	commande.Stdin = strings.NewReader(appDependencyCheckProgram)
	if sortie, err := commande.CombinedOutput(); err != nil {
		t.Fatalf("python invalide : %v\n%s", err, sortie)
	}
}
