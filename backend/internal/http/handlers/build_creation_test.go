package handlers

import (
	"strings"
	"testing"
)

// The interface writes a Dockerfile and asks the platform to build it. It has
// no repository to name and no way to know the registry this installation
// pushes to, and the API refused with "projectId, gitRepository and
// destinationImage are required" - a message naming three fields, two of which
// the person could not have provided.
func TestTheDestinationIsDerivedFromTheInstallationsOwnRegistry(t *testing.T) {
	h := Handlers{workspaceVSCodeImage: "harbor.emse.local/noryx-environments/noryx-vscode:0.1.2"}

	image, err := h.deriveEnvironmentImage("16108a6a-4dff-43f5-bafe-af0cb4177f04", "Training GPU")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(image, "harbor.emse.local/noryx-environments/") {
		t.Errorf("the image must go to the registry this cluster already pulls from: %s", image)
	}
	if !strings.Contains(image, "training-gpu") {
		t.Errorf("the environment's name should be readable in the image: %s", image)
	}
	// Two projects building an environment of the same name must not collide.
	other, err := h.deriveEnvironmentImage("d22c1476-9819-4902-abd5-0f47438ccf9f", "Training GPU")
	if err != nil {
		t.Fatal(err)
	}
	if strings.SplitN(image, ":", 2)[0] == strings.SplitN(other, ":", 2)[0] {
		t.Errorf("two projects share a repository: %s and %s", image, other)
	}
}

func TestAnInstallationWithNoRegistrySaysSo(t *testing.T) {
	h := Handlers{}
	if _, err := h.deriveEnvironmentImage("project", "name"); err == nil {
		t.Error("an installation with no environment image configured should refuse rather than build nowhere")
	}
}

func TestANameThatIsNotARepositoryPathIsReduced(t *testing.T) {
	for input, expected := range map[string]string{
		"Training GPU": "training-gpu",
		"  spaces  ":   "spaces",
		"Accentué!":    "accentu",
		"UPPER_snake":  "upper-snake",
		"--dashes--":   "dashes",
	} {
		if got := environmentSlug(input); got != expected {
			t.Errorf("%q became %q, expected %q", input, got, expected)
		}
	}
	if environmentSlug("!!!") != "" {
		t.Error("a name with nothing usable in it must reduce to nothing, so the caller is told")
	}
}

// A rebuild names the environment's repository and lets the platform tag it.
// Keying an environment on the full reference made every rebuild a new
// environment: the list filled with what is one thing built twice.
func TestARebuildStaysInTheSameRepository(t *testing.T) {
	if got := imageRepository("harbor.emse.local/noryx-environments/proj-training:1788808618"); got != "harbor.emse.local/noryx-environments/proj-training" {
		t.Errorf("the tag should be dropped, got %s", got)
	}
	if got := imageRepository("harbor.emse.local/noryx-environments/proj-training@sha256:abc"); got != "harbor.emse.local/noryx-environments/proj-training" {
		t.Errorf("a digest should be dropped, got %s", got)
	}
	// A registry with a port must not lose it: the colon there is not a tag.
	if got := imageRepository("registry.local:5000/team/image"); got != "registry.local:5000/team/image" {
		t.Errorf("a registry port is not a tag, got %s", got)
	}
	if got := imageRepository("registry.local:5000/team/image:v2"); got != "registry.local:5000/team/image" {
		t.Errorf("the tag should be dropped and the port kept, got %s", got)
	}
}

// The name somebody typed is what the screen shows. Without it the only
// identity left is the image reference.
func TestTheEnvironmentShowsTheNameSomebodyTyped(t *testing.T) {
	if got := environmentDisplayName("test-stef", "harbor/x/1cf6b279-114-test-stef:1788808618"); got != "test-stef" {
		t.Errorf("expected the typed name, got %s", got)
	}
	// Built before the platform remembered names: fall back to the reference.
	if got := environmentDisplayName("", "harbor/x/noryx-vscode:0.1.2"); got != "noryx-vscode:0.1.2" {
		t.Errorf("expected the image as a fallback, got %s", got)
	}
}
