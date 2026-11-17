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
	venv := workspaceProjectVenvPath
	file := workspaceRequirementsFile
	log := "/tmp/noryx-requirements.log"

	return []string{
		fmt.Sprintf("if [ -f %s ]; then", file),
		fmt.Sprintf("  echo '[bootstrap] requirements detected at %s'", file),
		// Who owns the venv and who is running: the two facts that explain
		// most of these failures, printed before the failure rather than
		// deduced from it afterwards.
		fmt.Sprintf("  if [ -d %s ] && [ ! -w %s ]; then", venv, venv),
		fmt.Sprintf("    echo \"[bootstrap] the project venv at %s is not writable by $(id -un) (uid $(id -u)); it belongs to $(stat -c '%%U' %s 2>/dev/null || echo unknown)\"", venv, venv),
		"  fi",
		fmt.Sprintf("  if python3 -m venv %s > /tmp/noryx-venv.log 2>&1; then", venv),
		"    :",
		"  else",
		fmt.Sprintf("    echo '[bootstrap] the project venv could not be created or updated:'"),
		"    tail -n 3 /tmp/noryx-venv.log 2>/dev/null | sed 's/^/[bootstrap]   /'",
		"  fi",
		"  venv_has_requirements=false",
		fmt.Sprintf("  if [ -x %s/bin/pip ]; then", venv),
		fmt.Sprintf("    if %s/bin/pip install --disable-pip-version-check -r %s > %s 2>&1; then", venv, file, log),
		"      venv_has_requirements=true",
		"      echo '[bootstrap] requirements installed into the project venv'",
		"    else",
		"      echo '[bootstrap] pip failed for the project venv:'",
		fmt.Sprintf("      tail -n 5 %s 2>/dev/null | sed 's/^/[bootstrap]   /'", log),
		"    fi",
		"  else",
		fmt.Sprintf("    echo '[bootstrap] no usable pip in %s'", venv),
		"  fi",
		fmt.Sprintf("  if python3 -m pip install --disable-pip-version-check --user -r %s >> %s 2>&1; then", file, log),
		"    echo '[bootstrap] requirements installed for the user as well'",
		"  else",
		"    echo '[bootstrap] pip failed for the user site packages:'",
		fmt.Sprintf("    tail -n 5 %s 2>/dev/null | sed 's/^/[bootstrap]   /'", log),
		"  fi",
		// The sentence that would have saved an evening: the launcher uses the
		// venv's interpreter, which does not read the user site.
		fmt.Sprintf("  if [ \"$venv_has_requirements\" != true ] && [ -x %s/bin/python3 ]; then", venv),
		fmt.Sprintf("    echo '[bootstrap] WARNING: the workload will start with %s/bin/python3, which does not read the user site packages - imports from %s will fail'", venv, file),
		"  fi",
		fmt.Sprintf("  if [ -x %s/bin/python ]; then", venv),
		fmt.Sprintf("    %s/bin/python -m pip list --format=freeze > /tmp/noryx-requirements-installed.txt 2>/dev/null || true", venv),
		"  fi",
		"  python3 -m pip list --format=freeze > /tmp/noryx-requirements-user-installed.txt 2>/dev/null || true",
		"else",
		fmt.Sprintf("  echo '[bootstrap] no requirements file found at %s'", file),
		"fi",
	}
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
