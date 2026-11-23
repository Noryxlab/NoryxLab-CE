package handlers

import (
	"os"
	"strings"
	"testing"
)

// The script used to print "requirements installation completed" whatever
// happened, because every pip call ended in `|| true`. An application whose
// venv was owned by root installed nothing, was congratulated, and died on an
// import three lines later. These tests hold the words that make that
// impossible to repeat.
func TestTheInstallNeverClaimsASuccessItHasNotSeen(t *testing.T) {
	script := strings.Join(requirementsBootstrapLines(), "\n")

	if strings.Contains(script, "requirements installation completed") {
		t.Fatal("the unconditional success line is back")
	}
	for _, forbidden := range []string{
		"pip install --disable-pip-version-check -r " + workspaceRequirementsFile + " > /tmp/noryx-requirements.log 2>&1 || true",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("an install is swallowed again: %s", forbidden)
		}
	}
	// What it must say instead: a branch on pip's exit code, and pip's own
	// words in the log the operator is actually reading.
	for _, expected := range []string{
		// The venv is named by the variable, so the log says which directory
		// was actually used rather than the one the code was written against.
		"requirements installed into $NORYX_PROJECT_VENV",
		"pip failed for the project venv:",
		"pip failed for the user site packages:",
		"tail -n 5 /tmp/noryx-requirements.log",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("missing %q", expected)
		}
	}
}

// The failure that cost the evening was not pip's: it was that the launcher
// runs the venv's interpreter, which does not read what `--user` installed.
// Nothing said so, and the crash two lines later said only "No module named".
func TestItWarnsThatTheLauncherWillNotSeeUserPackages(t *testing.T) {
	script := strings.Join(requirementsBootstrapLines(), "\n")
	if !strings.Contains(script, "does not read the user site packages") {
		t.Fatal("the warning that explains the import failure is missing")
	}
	if !strings.Contains(script, "is not writable by") {
		t.Fatal("an unwritable venv must name its owner and the user running")
	}
}

// Applications and jobs had the same block, copied. The same fix then had to
// be applied twice, and twice it was applied once.
func TestApplicationsAndJobsShareTheSameInstall(t *testing.T) {
	shared := strings.Join(requirementsBootstrapLines(), "\n")
	app := appBootstrapScript(8080, []string{"python3", "-m", "http.server"}, nil, extractMount{})
	job := jobBootstrapScript([]string{"python3", "-c", "print(1)"}, nil, extractMount{})
	for name, script := range map[string]string{"app": app, "job": job} {
		if !strings.Contains(script, shared) {
			t.Fatalf("the %s bootstrap no longer carries the shared install", name)
		}
	}
}

// Un venv qu'on ne peut pas ecrire n'est pas un venv a reutiliser.
//
// Deux pannes, deux causes, un seul symptome. En septembre le venv avait ete
// cree par root et la charge tournait en uid 1000 : pip ne pouvait pas y
// ecrire. Le 2026-11-23, l'image derriere `noryx-runtime:0.1.0` avait ete
// reconstruite et n'avait plus `/usr/bin/python3`, vers lequel pointaient les
// liens du venv - inutilisable, et appartenant a root, donc irreparable sur
// place.
//
// Le bootstrap signalait le cas et appelait quand meme `python3 -m venv`
// dessus, ce qui echouait. Il prend maintenant un venv a lui.
func TestUnVenvQuOnNePeutPasEcrireEstRemplaceEtNonSubi(t *testing.T) {
	script := strings.Join(requirementsBootstrapLines(), "\n")

	for _, expected := range []string{
		// La condition : le dossier existe et n'est pas ecrivable.
		"if [ -d /mnt/.venv ] && [ ! -w /mnt/.venv ]; then",
		// Le repli, nomme sur l'uid : deux personnes dans un projet sont un
		// cas legitime, pas seulement cet incident.
		"NORYX_PROJECT_VENV=/mnt/.venv-$(id -u)",
		// Et la raison, dite avant l'echec plutot que deduite apres.
		"is not writable by $(id -un)",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("missing %q in:\n%s", expected, script)
		}
	}

	// Ce qu'il ne doit surtout pas faire : supprimer le venv de quelqu'un
	// d'autre pour se faire de la place. Il peut etre en cours d'utilisation
	// par une charge qui tourne.
	for _, forbidden := range []string{"rm -rf /mnt/.venv", "rm -r /mnt/.venv"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("the bootstrap deletes another user's venv: %s", forbidden)
		}
	}
}

// Le chemin retenu est celui qui est installe ET celui qui va sur le PATH.
//
// C'est l'invariant que la variable existe pour tenir : le chemin etait une
// constante Go interpolee aux deux endroits, ce qui rendait tout repli
// inexprimable - une charge qui ne pouvait pas utiliser /mnt/.venv n'avait
// nulle part d'autre a aller.
func TestLeVenvInstalleEstCeluiQuiVaSurLePath(t *testing.T) {
	script := strings.Join(requirementsBootstrapLines(), "\n")
	if !strings.Contains(script, `python3 -m venv "$NORYX_PROJECT_VENV"`) {
		t.Error("the venv is not created at the chosen path")
	}
	if !strings.Contains(script, `"$NORYX_PROJECT_VENV/bin/pip" install`) {
		t.Error("pip does not install into the chosen venv")
	}
	if !strings.Contains(script, "export NORYX_PROJECT_VENV") {
		t.Error("the chosen path is not exported, so a launch line cannot read it")
	}
}

// Une seule copie de ce bloc, et un test qui le dit.
//
// Le commentaire de la fonction annonce « written once for applications and
// jobs » - et la copie vivait encore dans workspaces.go, avec les `|| true`
// et le « completed » que celle-ci avait corriges. Une correction appliquee a
// une moitie d'un code duplique est une correction qu'on refera.
func TestLeBlocDesRequirementsNExistePasEnDouble(t *testing.T) {
	for _, name := range []string{"workspaces.go", "apps.go", "jobs.go"} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		// Ligne a ligne, et les commentaires ecartes : ce fichier-ci parle de
		// la copie pour expliquer pourquoi elle a disparu, et le test ne doit
		// pas confondre une explication avec une recidive.
		for number, line := range strings.Split(string(source), "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue
			}
			// La signature de la copie : un pip qui installe LE fichier de
			// requirements du projet. Installer celui des exemples livres est
			// autre chose et reste legitime.
			//
			// Les deux formes, parce que la source porte l'identifiant la ou
			// la premiere version de ce test cherchait la valeur - elle ne
			// trouvait donc jamais rien, et declarait la duplication absente
			// alors qu'elle etait sous ses yeux.
			namesTheProjectFile := strings.Contains(code, workspaceRequirementsFile) ||
				strings.Contains(code, "workspaceRequirementsFile")
			if strings.Contains(code, "pip install") && namesTheProjectFile {
				t.Errorf("%s:%d installs the project requirements itself; call requirementsBootstrapLines() instead",
					name, number+1)
			}
			if strings.Contains(code, "requirements installation completed") {
				t.Errorf("%s:%d is back to announcing a success it has not seen", name, number+1)
			}
		}
	}
}
