package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func TestProjectOwnerHasMembershipWithoutExplicitRole(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("stef", "Owned project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}

	h := Handlers{
		projectStore: projects,
		accessStore:  memory.NewAccessStore(),
	}
	if !h.hasProjectMembership("stef", item.ID) {
		t.Fatal("project owner must have project membership")
	}
	if h.hasProjectMembership("other-user", item.ID) {
		t.Fatal("unrelated user must not have project membership")
	}
}

// Capacity moved off the launch form and onto the project. The point of the
// move is that the value actually comes from the project, so that is what this
// checks - a project setting that is stored but not read would look identical
// on screen and change nothing about the volume a workspace gets.
func TestWorkspaceStorageComesFromTheProject(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("stef", "Sizeable project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	if err := projects.UpdateWorkspaceStorageSize(item.ID, "50Gi"); err != nil {
		t.Fatal(err)
	}

	stored, found, err := Handlers{projectStore: projects}.projectByID(item.ID)
	if err != nil || !found {
		t.Fatalf("the project must be readable back: found=%v err=%v", found, err)
	}
	if stored.WorkspaceStorageSize != "50Gi" {
		t.Fatalf("the project must carry its workspace capacity, got %q", stored.WorkspaceStorageSize)
	}

	// "default" is how the interface says "follow the platform", because JSON
	// cannot distinguish an empty string from an absent field.
	if err := projects.UpdateWorkspaceStorageSize(item.ID, ""); err != nil {
		t.Fatal(err)
	}
	cleared, _, _ := Handlers{projectStore: projects}.projectByID(item.ID)
	if cleared.WorkspaceStorageSize != "" {
		t.Fatalf("clearing the setting must fall back to the platform default, got %q", cleared.WorkspaceStorageSize)
	}
}
