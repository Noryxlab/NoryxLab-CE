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
		"Training GPU":  "training-gpu",
		"  spaces  ":    "spaces",
		"Accentué!":     "accentu",
		"UPPER_snake":   "upper-snake",
		"--dashes--":    "dashes",
	} {
		if got := environmentSlug(input); got != expected {
			t.Errorf("%q became %q, expected %q", input, got, expected)
		}
	}
	if environmentSlug("!!!") != "" {
		t.Error("a name with nothing usable in it must reduce to nothing, so the caller is told")
	}
}
