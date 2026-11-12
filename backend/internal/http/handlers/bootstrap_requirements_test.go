package handlers

import (
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
		"requirements installed into the project venv",
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
