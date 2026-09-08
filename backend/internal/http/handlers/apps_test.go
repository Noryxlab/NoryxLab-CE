package handlers

import (
	"strings"
	"testing"
)

func TestNormalizeAppSlugRemovesAccents(t *testing.T) {
	tests := map[string]string{
		"Météo Couilly":        "meteo-couilly",
		"Évaluation de modèle": "evaluation-de-modele",
		" déjà--propre ":       "deja-propre",
	}
	for input, expected := range tests {
		if got := normalizeAppSlug(input); got != expected {
			t.Fatalf("normalizeAppSlug(%q) = %q, want %q", input, got, expected)
		}
	}
}

// An app must be able to run what the platform just installed for it.
//
// The bootstrap installed requirements into the project venv and into
// ~/.local, then ran the launch command with neither on PATH - so a project
// whose requirements.txt held streamlit died with "streamlit: not found",
// after installing streamlit. The workspace bootstrap had always exported it;
// the app one never did, which is why the same file worked in one and not the
// other.
func TestAppBootstrapRunsWhatItInstalled(t *testing.T) {
	script := appBootstrapScript(8501, "streamlit run app.py --server.port 8501", nil)
	for _, expected := range []string{
		"export PATH=/mnt/.venv/bin:$PATH",
		"export PATH=$HOME/.local/bin:$PATH",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("app bootstrap missing %q:\n%s", expected, script)
		}
	}
	// The launch command becomes the container's process, so a stop signal
	// reaches the server rather than the shell that started it.
	if !strings.Contains(script, "exec streamlit run app.py") {
		t.Fatalf("the launch command is not exec'd:\n%s", script)
	}
}
