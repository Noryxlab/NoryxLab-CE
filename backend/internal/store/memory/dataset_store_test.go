package memory

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

func TestDatasetOrganizationOwnershipAndAccess(t *testing.T) {
	store := NewDatasetStore()
	item := dataset.New("stef", "example", "", "", "", "minio", "non-hds", "", "")
	if err := store.Create(item); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateOwner(item.ID, "organization", "imt"); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListBySubjects([]dataset.Subject{{Type: "organization", ID: "imt"}})
	if err != nil || len(items) != 1 || items[0].AccessRole != "owner" {
		t.Fatalf("expected organization owner access, items=%v err=%v", items, err)
	}

	access := dataset.Access{DatasetID: item.ID, SubjectType: "organization", SubjectID: "partners", Role: "reader"}
	if err := store.SetAccess(access); err != nil {
		t.Fatal(err)
	}
	items, err = store.ListBySubjects([]dataset.Subject{{Type: "organization", ID: "partners"}})
	if err != nil || len(items) != 1 || items[0].AccessRole != "reader" {
		t.Fatalf("expected organization reader access, items=%v err=%v", items, err)
	}
}
