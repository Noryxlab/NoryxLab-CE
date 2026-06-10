package memory

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

func TestProjectStoreUpdatesOwner(t *testing.T) {
	items := NewProjectStore()
	item := project.NewOwned("stef", "Owned project", "")
	if err := items.Create(item); err != nil {
		t.Fatal(err)
	}
	if err := items.UpdateOwner(item.ID, "organization", "imt"); err != nil {
		t.Fatal(err)
	}

	projects, err := items.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].OwnerType != "organization" || projects[0].OwnerID != "imt" {
		t.Fatalf("unexpected updated project: %#v", projects)
	}
}
