package handlers

import (
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// The tag is the revision number, not a Unix timestamp: it is what the screen
// shows and what somebody says out loud.
func TestTheTagIsTheRevisionNumber(t *testing.T) {
	builds := memory.NewBuildStore()
	h := Handlers{buildStore: builds}
	repository := "harbor.emse.local/noryx-environments/proj-training"

	if got := h.nextRevisionTag("project", repository); got != "r1" {
		t.Errorf("the first build is r1, got %s", got)
	}
	for _, tag := range []string{"r1", "r2"} {
		if err := builds.Create(build.New("project", "", "", "Dockerfile", ".", repository+":"+tag, "job-"+tag)); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.nextRevisionTag("project", repository); got != "r3" {
		t.Errorf("after two revisions the next is r3, got %s", got)
	}
	// Another project's environment of the same name counts separately.
	if got := h.nextRevisionTag("other-project", repository); got != "r1" {
		t.Errorf("another project starts at r1, got %s", got)
	}
	// And another environment in the same project.
	if got := h.nextRevisionTag("project", "harbor.emse.local/noryx-environments/proj-other"); got != "r1" {
		t.Errorf("another environment starts at r1, got %s", got)
	}
}

func TestTheDerivedRepositoryCarriesNoTag(t *testing.T) {
	h := Handlers{workspaceVSCodeImage: "harbor.emse.local/noryx-environments/noryx-vscode:0.1.2"}
	image, err := h.deriveEnvironmentImage("project-id", "Training GPU")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.TrimPrefix(image, "harbor.emse.local"), ":") {
		t.Errorf("the derived value is a repository, the tag comes from the revision: %s", image)
	}
}
