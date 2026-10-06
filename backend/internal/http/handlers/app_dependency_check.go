package handlers

import (
	"fmt"
	"strings"
)

// The dependency check an application runs before it starts.
//
// Why it exists. On 2026-10-06 a Streamlit application that had been serving
// since September stopped working the moment its pod was recreated, with a
// Python traceback in the browser: ModuleNotFoundError: No module named
// 'plotly'. The script imported plotly, requirements.txt did not declare it,
// and the three days between their modification dates say what happened -
// somebody installed it by hand in the running container and it worked for a
// month. A pod is not a disk. At the next start the platform replays
// requirements.txt and nothing else, and the package is gone.
//
// What the person saw was a stack trace from Streamlit. What the platform
// knew, and did not say, is that the file it had just installed from was
// missing a line. That gap is the whole point of this check.
//
// Two directions, and the second is the one that would have prevented it:
//
//   - an import nothing can satisfy. The application is going to crash on its
//     first request; saying which module and which file beats a traceback.
//   - an import satisfied by something requirements.txt does not declare. The
//     application works right now and will break at its next restart, which
//     may be months away and will look like an unrelated platform fault. This
//     is the warning that had been available every single start since
//     September.
//
// It never fails the launch. An application that mostly works is worth more to
// its owner than a refusal, and a check that can take an application down is a
// check somebody will want removed. It writes, loudly, and gets out of the way.
const appDependencyCheckProgram = `import ast, importlib.util, pathlib, re, sys

# Les noms d'import qui ne portent pas le nom de leur paquet. Il n'y a pas de
# table generale : seul le paquet installe sait ce qu'il fournit, et un paquet
# absent ne peut rien dire. Les plus courants, donc, et le nom du module sinon -
# une suggestion presque juste vaut mieux qu'aucune.
PAQUETS = {
    "cv2": "opencv-python", "PIL": "pillow", "sklearn": "scikit-learn",
    "yaml": "PyYAML", "bs4": "beautifulsoup4", "dateutil": "python-dateutil",
    "dotenv": "python-dotenv", "fitz": "pymupdf", "serial": "pyserial",
    "OpenSSL": "pyOpenSSL", "Crypto": "pycryptodome", "psycopg2": "psycopg2-binary",
    "pydicom": "pydicom", "skimage": "scikit-image", "google": "google-api-python-client",
    "jwt": "PyJWT", "docx": "python-docx", "pptx": "python-pptx", "magic": "python-magic",
    "zoneinfo": "", "attr": "attrs", "pkg_resources": "setuptools",
}


def modules_importes(chemin):
    """Les modules racine importes au premier niveau du fichier.

    ast et non une expression reguliere : un "import" dans une chaine de
    caracteres ou un commentaire n'est pas un import, et le signaler enverrait
    quelqu'un chercher un paquet dont il n'a pas besoin.
    """
    try:
        arbre = ast.parse(pathlib.Path(chemin).read_text(encoding="utf-8", errors="replace"))
    except Exception:
        # Un fichier illisible ou invalide n'est pas notre sujet : l'interprete
        # le dira mieux que nous, et refuser de demarrer pour autant serait
        # pire que se taire.
        return []
    trouves = []
    for noeud in ast.walk(arbre):
        if isinstance(noeud, ast.Import):
            for alias in noeud.names:
                trouves.append(alias.name.split(".")[0])
        elif isinstance(noeud, ast.ImportFrom):
            # Un import relatif ne vient pas d'un paquet : il vient du projet.
            if noeud.level == 0 and noeud.module:
                trouves.append(noeud.module.split(".")[0])
    return trouves


def normalise(nom):
    return re.sub(r"[-_.]+", "-", nom).lower()


def declares(chemin):
    """Les distributions nommees dans requirements.txt."""
    noms = set()
    try:
        lignes = pathlib.Path(chemin).read_text(encoding="utf-8", errors="replace").splitlines()
    except Exception:
        return noms
    for ligne in lignes:
        ligne = ligne.split("#", 1)[0].strip()
        if not ligne or ligne.startswith("-"):
            continue
        # On coupe au premier caractere qui n'appartient pas au nom : version,
        # extras, marqueur d'environnement.
        nom = re.split(r"[\[<>=!~;\s]", ligne, 1)[0].strip()
        if nom:
            noms.add(normalise(nom))
    return noms


def distribution_de(module):
    """Le paquet qui fournit ce module, s'il est installe."""
    try:
        from importlib.metadata import packages_distributions
        for nom in packages_distributions().get(module, []):
            return nom
    except Exception:
        pass
    return PAQUETS.get(module) or module


def main():
    if len(sys.argv) < 3:
        return
    entree, requirements = sys.argv[1], sys.argv[2]
    if not pathlib.Path(entree).is_file():
        return

    standard = getattr(sys, "stdlib_module_names", frozenset())
    attendus = declares(requirements)
    a_un_fichier = pathlib.Path(requirements).is_file()

    manquants, non_declares = [], []
    vus = set()
    for module in modules_importes(entree):
        if module in vus or module in standard or module == "__future__":
            continue
        vus.add(module)
        try:
            trouve = importlib.util.find_spec(module) is not None
        except Exception:
            trouve = False
        if not trouve:
            manquants.append((module, PAQUETS.get(module) or module))
            continue
        # Importable. Est-ce parce qu'on l'a demande, ou par accident ?
        distribution = distribution_de(module)
        if distribution and normalise(distribution) not in attendus:
            non_declares.append((module, distribution))

    nom = pathlib.Path(entree).name
    for module, paquet in manquants:
        print(f"[deps] MANQUANT  {nom} importe '{module}' et rien ne le fournit.")
        print(f"[deps]           L'application va echouer. Ajoutez '{paquet}' a {requirements}.")
    for module, paquet in non_declares:
        print(f"[deps] FRAGILE   {nom} importe '{module}', fourni par '{paquet}',")
        if a_un_fichier:
            print(f"[deps]           que {requirements} ne declare pas. Ca marche maintenant")
        else:
            print(f"[deps]           et il n'y a pas de {requirements}. Ca marche maintenant")
        print("[deps]           et cassera au prochain redemarrage du conteneur,")
        print(f"[deps]           qui ne rejoue que {requirements}.")
    if not manquants and not non_declares:
        print(f"[deps] les imports de {nom} sont tous declares.")


main()
`

// appDependencyCheckLines emits the bootstrap lines that run the check.
//
// Empty when there is no Python entry file to look at, which is the honest
// answer for a static server or a shell entrypoint: there is nothing to read
// imports from, and printing a reassuring line about a file we never opened
// would be worse than silence.
//
// The entry file is taken from the launch command because that is where it is
// actually stated - `streamlit run weather_app.py` - rather than guessed by
// scanning the project, which would warn about every script somebody left
// lying around and teach the owner to ignore the warnings.
func appDependencyCheckLines(launchArgv []string) []string {
	entry := pythonEntryFile(launchArgv)
	if entry == "" {
		return nil
	}
	const programPath = "/tmp/noryx-check-deps.py"
	lines := []string{
		// No apostrophe in this message, on purpose: it is inside a
		// single-quoted shell string, and "the entry file's imports" would end
		// that string three words early and leave the rest as shell syntax.
		"echo '[bootstrap] checking the imports of the entry file against " + workspaceRequirementsFile + "'",
		// A quoted heredoc marker, so the shell expands nothing inside the
		// program: it contains $, backticks and quotes, and every one of them
		// means something to a shell and nothing to Python.
		"cat > " + programPath + " <<'NORYX_DEPS_EOF'",
		appDependencyCheckProgram,
		"NORYX_DEPS_EOF",
		// Never fatal. See the comment on the program above: a check that can
		// stop an application is a check that gets deleted.
		fmt.Sprintf("python3 %s %s %s || true",
			programPath, shellQuote(entry), shellQuote(workspaceRequirementsFile)),
	}
	return lines
}

// pythonEntryFile picks the script the application runs, or "".
//
// The first word ending in .py, which covers the forms people write:
// `streamlit run app.py`, `python3 main.py`, `python -m streamlit run app.py`.
// A relative name is resolved against the project directory, which is where
// the launch command runs from.
func pythonEntryFile(launchArgv []string) string {
	for _, word := range launchArgv {
		word = strings.TrimSpace(word)
		// A single word carrying spaces is a command line, not an argv - the
		// same case the launch builder handles - so it is split here too
		// rather than being rejected for containing no bare .py word.
		for _, piece := range strings.Fields(word) {
			piece = strings.Trim(piece, `"'`)
			if !strings.HasSuffix(piece, ".py") {
				continue
			}
			if strings.HasPrefix(piece, "/") {
				return piece
			}
			return workspaceProjectMountPath + "/" + strings.TrimPrefix(piece, "./")
		}
	}
	return ""
}
