package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Le script de bootstrap, execute pour de vrai.
//
// Comparer des chaines aurait dit que la ligne est la ; ce qu'on veut savoir
// est si le shell prend la bonne branche. Les deux cas qui comptent sont
// opposes et un test de chaine ne distingue ni l'un ni l'autre : un venv sain
// ne doit pas etre refait (1 805 ms rendus a chaque demarrage), un venv dont
// l'interpreteur a disparu doit l'etre (l'incident du 23/11/2026).
func runBootstrap(t *testing.T, venvPython string) (sortie string, venvRefait bool) {
	t.Helper()
	racine := t.TempDir()
	venv := filepath.Join(racine, "venv")
	stubs := filepath.Join(racine, "stubs")
	marqueur := filepath.Join(racine, "venv-refait")
	for _, dossier := range []string{filepath.Join(venv, "bin"), stubs, filepath.Join(racine, "mnt")} {
		if err := os.MkdirAll(dossier, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Le python du venv : present et fonctionnel, ou present et casse - un
	// lien symbolique vers un interpreteur qui n'existe plus, exactement la
	// forme qu'avait l'incident.
	if venvPython != "" {
		ecrire(t, filepath.Join(venv, "bin", "python"), venvPython)
	}
	ecrire(t, filepath.Join(venv, "bin", "pip"), "#!/bin/sh\nexit 0\n")

	// python3 sur le PATH : note qu'on lui a demande de refaire le venv.
	ecrire(t, filepath.Join(stubs, "python3"), "#!/bin/sh\n"+
		"case \"$*\" in *'-m venv'*) : > "+marqueur+" ;; esac\nexit 0\n")

	script := filepath.Join(racine, "bootstrap.sh")
	lignes := requirementsBootstrapLines()
	// Le chemin du venv est fixe par la selection ; on le remplace par le
	// notre, qui est ce que la selection aurait choisi.
	corps := strings.Join(lignes, "\n")
	corps = strings.Replace(corps, projectVenvEnvVar+"="+workspaceProjectVenvPath,
		projectVenvEnvVar+"="+venv, 1)
	corps = strings.ReplaceAll(corps, workspaceRequirementsFile, filepath.Join(racine, "mnt", "requirements.txt"))
	ecrire(t, script, "#!/bin/sh\n"+corps+"\n")
	ecrire(t, filepath.Join(racine, "mnt", "requirements.txt"), "pandas\n")

	commande := exec.Command("/bin/sh", script)
	commande.Env = append(os.Environ(), "PATH="+stubs+string(os.PathListSeparator)+os.Getenv("PATH"))
	resultat, _ := commande.CombinedOutput()

	_, err := os.Stat(marqueur)
	return string(resultat), err == nil
}

func ecrire(t *testing.T, chemin, contenu string) {
	t.Helper()
	if err := os.WriteFile(chemin, []byte(contenu), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUnVenvQuiRepondNEstPasRefait(t *testing.T) {
	sortie, refait := runBootstrap(t, "#!/bin/sh\nexit 0\n")
	if refait {
		t.Error("le venv a ete refait alors qu'il repondait : 1,8 s perdues a chaque demarrage")
	}
	if !strings.Contains(sortie, "answers, reusing it") {
		t.Errorf("le journal ne dit pas pourquoi rien n'a ete refait :\n%s", sortie)
	}
}

// La reparation qui justifiait l'appel inconditionnel doit survivre : un lien
// vers un interpreteur disparu ne s'execute pas, donc le venv est refait.
func TestUnInterpreteurDisparuFaitRefaireLeVenv(t *testing.T) {
	sortie, refait := runBootstrap(t, "#!/bin/sh\nexit 127\n")
	if !refait {
		t.Errorf("le venv casse n'a pas ete repare :\n%s", sortie)
	}
	if !strings.Contains(sortie, "created or repaired") {
		t.Errorf("le journal ne dit pas que le venv a ete refait :\n%s", sortie)
	}
}

// Et un venv qui n'existe pas du tout est cree, ce qui est le premier
// demarrage de n'importe quel projet.
func TestUnVenvAbsentEstCree(t *testing.T) {
	_, refait := runBootstrap(t, "")
	if !refait {
		t.Error("aucun venv n'a ete cree au premier demarrage")
	}
}
