package handlers

import "fmt"

// The requirements install, written once for applications and jobs.
//
// It was written twice, identically, and said the same untrue thing in both
// places: every pip invocation ended in `|| true`, its output went to a file
// inside a container about to exit, and the script then printed
// "[bootstrap] requirements installation completed" whatever had happened.
//
// On 2026-11-12 an application that had been serving for three weeks was
// restarted with its node and died on `No module named uvicorn`, three lines
// under that word "completed". The cause was two months old: the project venv
// had been created by root in September, the workload runs as uid 1000, and
// pip could not write into it. The `--user` fallback succeeded - into a site
// the launcher's interpreter does not read, because the launcher runs the
// venv's python. Every piece of that was knowable at launch, and the log said
// the opposite.
//
// So nothing here claims a success it has not seen. A failure names what
// failed, shows the last lines of pip's own output, and - when the venv is the
// part that broke - says plainly that the application is about to start
// without those packages, because that is the sentence somebody reading a
// crash needs above the traceback rather than inside a deleted file.
//
// What it still does not do is stop. A job whose requirements are optional
// must keep its exit code, and an application that fails to import will say so
// itself, now with the reason printed above it.
func requirementsBootstrapLines() []string {
	file := workspaceRequirementsFile
	log := "/tmp/noryx-requirements.log"

	return append(projectVenvSelectionLines(), []string{
		fmt.Sprintf("if [ -f %s ]; then", file),
		fmt.Sprintf("  echo '[bootstrap] requirements detected at %s'", file),
		// Recreer le venv sur le chemin retenu, et non sur le chemin par
		// defaut : la selection au-dessus a deja ecarte celui qu'on ne peut
		// pas ecrire. L'appel reste inconditionnel, parce que sur un venv
		// qu'on possede il repare justement l'interpreteur qui a bouge.
		fmt.Sprintf("  if python3 -m venv \"$%s\" > /tmp/noryx-venv.log 2>&1; then", projectVenvEnvVar),
		"    :",
		"  else",
		"    echo '[bootstrap] the project venv could not be created or updated:'",
		"    tail -n 3 /tmp/noryx-venv.log 2>/dev/null | sed 's/^/[bootstrap]   /'",
		"  fi",
		"  venv_has_requirements=false",
		fmt.Sprintf("  if [ -x \"$%s/bin/pip\" ]; then", projectVenvEnvVar),
		fmt.Sprintf("    if \"$%s/bin/pip\" install --disable-pip-version-check -r %s > %s 2>&1; then", projectVenvEnvVar, file, log),
		"      venv_has_requirements=true",
		fmt.Sprintf("      echo \"[bootstrap] requirements installed into $%s\"", projectVenvEnvVar),
		"    else",
		"      echo '[bootstrap] pip failed for the project venv:'",
		fmt.Sprintf("      tail -n 5 %s 2>/dev/null | sed 's/^/[bootstrap]   /'", log),
		"    fi",
		"  else",
		fmt.Sprintf("    echo \"[bootstrap] no usable pip in $%s\"", projectVenvEnvVar),
		"  fi",
		fmt.Sprintf("  if python3 -m pip install --disable-pip-version-check --user -r %s >> %s 2>&1; then", file, log),
		"    echo '[bootstrap] requirements installed for the user as well'",
		"  else",
		"    echo '[bootstrap] pip failed for the user site packages:'",
		fmt.Sprintf("    tail -n 5 %s 2>/dev/null | sed 's/^/[bootstrap]   /'", log),
		"  fi",
		// The sentence that would have saved an evening: the launcher uses the
		// venv's interpreter, which does not read the user site.
		fmt.Sprintf("  if [ \"$venv_has_requirements\" != true ] && [ -x \"$%s/bin/python3\" ]; then", projectVenvEnvVar),
		fmt.Sprintf("    echo \"[bootstrap] WARNING: the workload will start with $%s/bin/python3, which does not read the user site packages - imports from %s will fail\"", projectVenvEnvVar, file),
		"  fi",
		fmt.Sprintf("  if [ -x \"$%s/bin/python\" ]; then", projectVenvEnvVar),
		fmt.Sprintf("    \"$%s/bin/python\" -m pip list --format=freeze > /tmp/noryx-requirements-installed.txt 2>/dev/null || true", projectVenvEnvVar),
		"  fi",
		"  python3 -m pip list --format=freeze > /tmp/noryx-requirements-user-installed.txt 2>/dev/null || true",
		"else",
		fmt.Sprintf("  echo '[bootstrap] no requirements file found at %s'", file),
		"fi",
	}...)
}

// projectVenvEnvVar is the shell variable that carries the venv actually in
// use, so the lines that install into it and the line that puts it on PATH
// cannot disagree.
//
// It exists because they could. The path was a Go constant interpolated in
// both places, which made a fallback impossible to express: a workload that
// could not use /mnt/.venv had nowhere else to go, and the launch line put a
// venv on PATH that the install had never reached.
const projectVenvEnvVar = "NORYX_PROJECT_VENV"

// projectVenvSelectionLines choose the venv this workload will use.
//
// The project venv lives on the project volume, which is the right place: a
// `requirements.txt` installed at start is the development loop, and asking
// somebody to rebuild an image to add a package would kill it. Production is
// what a built environment is for.
//
// But a cache on a persistent volume outlives the image it was built from,
// and twice now that has stopped an application:
//
//   - 2026-09: the venv was created by root, the workload now runs as uid
//     1000, and pip could not write into it.
//   - 2026-11-23: the image behind `noryx-runtime:0.1.0` was rebuilt and no
//     longer has /usr/bin/python3, which the venv's own symlinks point at. The
//     venv was unusable AND root-owned, so it could not be repaired in place
//     either.
//
// So the venv is reused only when this user can write it. When it exists and
// belongs to somebody else, the workload takes a venv of its own named after
// its uid rather than failing - two people in one project are a legitimate
// case too, not only this incident. And when the venv is writable but its
// interpreter has moved, nothing special is needed: `python3 -m venv` on a
// venv you own rewrites its symlinks, which is the repair.
//
// What is deliberately not done is deleting the other venv. It is somebody
// else's installed packages, it may be in use by a workload running right
// now, and a bootstrap that removes data to make room for itself is not a
// bootstrap anybody should trust.
func projectVenvSelectionLines() []string {
	venv := workspaceProjectVenvPath
	return []string{
		fmt.Sprintf("%s=%s", projectVenvEnvVar, venv),
		fmt.Sprintf("if [ -d %s ] && [ ! -w %s ]; then", venv, venv),
		fmt.Sprintf(`  echo "[bootstrap] the project venv at %s belongs to $(stat -c '%%U' %s 2>/dev/null || echo another user) and is not writable by $(id -un) (uid $(id -u))"`, venv, venv),
		fmt.Sprintf("  %s=%s-$(id -u)", projectVenvEnvVar, venv),
		fmt.Sprintf("  echo \"[bootstrap] using $%s instead, so this workload has a venv it owns\"", projectVenvEnvVar),
		"fi",
		fmt.Sprintf("export %s", projectVenvEnvVar),
	}
}

// projectVenvPathExpression is what a launch line puts on PATH: the venv the
// bootstrap chose, falling back on the default for a script that never ran
// the selection.
func projectVenvPathExpression() string {
	return fmt.Sprintf("${%s:-%s}", projectVenvEnvVar, workspaceProjectVenvPath)
}

// debMirrorBootstrapLines turns NORYX_DEB_MIRROR into a sources.list.
//
// apt is the one artifact leg with no environment form, so unlike pip and
// conda it cannot be configured by injecting a variable - the file has to
// exist. Writing it here rather than in the images keeps the platform out of
// the business of knowing which Debian an image is built on: the release name
// comes from /etc/os-release in the image itself.
//
// The default sources are set aside rather than left alongside. Leaving both
// would mean apt tries the archive first and **hangs** when a closed workload
// cannot reach it - the failure shape ADR-054 orders the work to avoid, and
// the one this project has already paid for three times. A workload that can
// reach the internet loses nothing by using the mirror.
//
// `[trusted=yes]` because Pulp publishes an unsigned Release file unless a
// signing service is configured. That is a real reduction and it is stated
// here rather than hidden: the mirror is reached over the cluster network at
// an address the platform declares, so the trust being placed is in that
// endpoint, not in a signature.
func debMirrorBootstrapLines() []string {
	return []string{
		`if [ -n "${NORYX_DEB_MIRROR:-}" ] && command -v sudo >/dev/null 2>&1; then`,
		`  codename=$(. /etc/os-release 2>/dev/null; echo "${VERSION_CODENAME:-}")`,
		`  if [ -z "$codename" ]; then`,
		`    echo "[bootstrap] apt mirror declared but this image does not name its release in /etc/os-release; sources left untouched"`,
		`  else`,
		`    base="${NORYX_DEB_MIRROR%/}"`,
		`    if printf '%s\n' \`,
		`      "deb [trusted=yes] $base/$codename $codename main" \`,
		`      "deb [trusted=yes] $base/$codename-updates $codename-updates main" \`,
		`      "deb [trusted=yes] $base/$codename-security $codename-security main" \`,
		`      | sudo tee /etc/apt/sources.list.d/noryx-mirror.list >/dev/null 2>&1; then`,
		`      sudo sh -c ': > /etc/apt/sources.list' 2>/dev/null || true`,
		`      sudo rm -f /etc/apt/sources.list.d/debian.sources 2>/dev/null || true`,
		`      echo "[bootstrap] apt points at $base for $codename; the image's own sources are set aside"`,
		`    else`,
		`      echo "[bootstrap] apt mirror declared but /etc/apt is not writable by $(id -un); sources left untouched"`,
		`    fi`,
		`  fi`,
		`fi`,
	}
}
