package handlers

import "testing"

// The image decides the kind.
//
// A launch form once showed one environment while holding another, sent that
// environment's image with the other's kind, and the platform refused a
// combination nobody had assembled - "not compatible with jupyter" under a
// field reading noryx-vscode. Asking a caller for both invites them to
// disagree; asking for one removes the question.
func TestTheKindComesFromTheImage(t *testing.T) {
	h := Handlers{
		workspaceVSCodeImage:  "registry.example/noryx-vscode:0.1.2",
		workspaceJupyterImage: "registry.example/noryx-jupyter:0.1.0",
		workspaceRStudioImage: "registry.example/noryx-rstudio:0.1.0",
	}
	for image, want := range map[string]string{
		"registry.example/noryx-vscode:0.1.2":  "vscode",
		"registry.example/noryx-jupyter:0.1.0": "jupyter",
		"registry.example/noryx-rstudio:0.1.0": "rstudio",
	} {
		if got := h.deriveIDEForImage(image); got != want {
			t.Errorf("%s -> %q, want %q", image, got, want)
		}
	}
}

func TestAnImageNobodyConfiguredStillAnswers(t *testing.T) {
	// A project builds its own environment and never registers it anywhere.
	// The name is what the catalogue reads too, so a launch gets the kind the
	// catalogue showed rather than a different answer to the same question.
	h := Handlers{}
	if got := h.deriveIDEForImage("registry.example/team/my-vscode-env:3"); got != "vscode" {
		t.Errorf("got %q, want vscode", got)
	}
}

func TestSilenceRatherThanAGuess(t *testing.T) {
	// Nothing in the name says what it runs. Empty means "no opinion", which
	// the caller turns into the platform default - as opposed to naming a kind
	// on no evidence and opening the wrong application.
	h := Handlers{}
	if got := h.deriveIDEForImage("registry.example/team/analysis:7"); got != "" {
		t.Errorf("got %q, want no opinion", got)
	}
	if got := h.deriveIDEForImage("  "); got != "" {
		t.Errorf("blank image produced %q", got)
	}
}
