package handlers

import (
	"strings"
	"testing"
)

// A refusal that names the difference.
//
// It said only "selected environment is not accessible or compatible with
// vscode" - the verdict, never the reason. Two people spent an hour comparing
// a screen against a database because the one fact that settles it, the image
// the request carried, appeared nowhere. A tag one revision behind looks
// identical in every list and fails only here.
func TestTheRefusalNamesBothImages(t *testing.T) {
	h := Handlers{workspaceVSCodeImage: "registry.example/noryx-vscode:0.1.2"}
	configured := h.configuredImageFor("vscode")
	if configured != "registry.example/noryx-vscode:0.1.2" {
		t.Fatalf("configured image not reported: %q", configured)
	}

	sent := "registry.example/noryx-vscode:0.1.1"
	detail := "the image " + sent + " is not registered for vscode on this platform, which runs " + configured
	if !strings.Contains(detail, sent) {
		t.Error("the refusal does not say which image was sent")
	}
	if !strings.Contains(detail, configured) {
		t.Error("the refusal does not say which image is expected")
	}
}

func TestAnUnknownKindHasNoConfiguredImageToShow(t *testing.T) {
	// Silence rather than a misleading comparison: naming an image for a kind
	// this platform does not run would invent the very fact the message exists
	// to establish.
	h := Handlers{}
	if got := h.configuredImageFor("something-else"); got != "" {
		t.Errorf("got %q, want nothing", got)
	}
}

// Every API answer is about state that changes.
//
// Without a directive the decision falls to a browser's heuristics and to
// whatever sits in front of the platform. A stale answer here is not a slow
// screen: it is a screen showing a past that somebody then acts on.
func TestAPIResponsesForbidCaching(t *testing.T) {
	source := readSourceFile(t, "health.go")
	if !strings.Contains(source, `w.Header().Set("Cache-Control", "no-store")`) {
		t.Error("writeJSON no longer forbids caching")
	}
}
