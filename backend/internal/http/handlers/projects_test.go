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
