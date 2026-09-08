package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Transferring a dataset to an organization you do not belong to made it
// vanish from your own screen while remaining yours to administer: reading it
// by id was allowed all along, only the listing disagreed. An administrator now
// sees it, and it is marked as visible through the role rather than a grant.
func TestAdministratorSeesWhatTheyDoNotHoldAGrantOn(t *testing.T) {
	datasets := memory.NewDatasetStore()
	mine := datasetdomain.New("stef", "Mine", "", "bucket-a", "", "s3", "standard", "", "")
	theirs := datasetdomain.New("someone-else", "HDS-Essilor", "", "bucket-b", "", "s3", "standard", "", "")
	theirs.OwnerType = "organization"
	theirs.OwnerID = "58c2770c"
	for _, item := range []datasetdomain.Dataset{mine, theirs} {
		if err := datasets.Create(item); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	h := Handlers{datasetStore: datasets, bootstrapAdminUser: "stef"}
	identity := auth.Identity{Username: "stef"}

	visible, err := h.datasetsVisibleTo(identity)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(visible) != 2 {
		t.Fatalf("administrator sees %d datasets, want both", len(visible))
	}
	for _, item := range visible {
		switch item.Name {
		case "Mine":
			if item.AdminVisible {
				t.Fatal("a dataset the administrator owns must not be marked as admin-only")
			}
		case "HDS-Essilor":
			if !item.AdminVisible {
				t.Fatal("a dataset visible only through the admin role must say so")
			}
		}
	}
}

// Everyone else keeps the view their grants give them.
func TestOrdinaryUserSeesOnlyTheirOwn(t *testing.T) {
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(datasetdomain.New("someone-else", "Theirs", "", "bucket-b", "", "s3", "standard", "", "")); err != nil {
		t.Fatalf("create: %v", err)
	}
	h := Handlers{datasetStore: datasets, bootstrapAdminUser: "stef"}

	visible, err := h.datasetsVisibleTo(auth.Identity{Username: "someone-without-rights"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("a user with no grant sees %d datasets, want none", len(visible))
	}
}
