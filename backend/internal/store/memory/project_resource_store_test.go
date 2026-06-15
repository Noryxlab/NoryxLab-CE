package memory

import "testing"

func TestListDatasourceProjectIDs(t *testing.T) {
	store := NewProjectResourceStore()
	if err := store.AttachDatasource("project-a", "datasource-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.AttachDatasource("project-b", "datasource-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.AttachDatasource("project-b", "datasource-2"); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListDatasourceProjectIDs("datasource-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two attached projects, got %#v", items)
	}
}
