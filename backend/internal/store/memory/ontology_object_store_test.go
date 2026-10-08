package memory

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Deleting an ontology takes its file list with it.
//
// It did not. The rows stayed, keyed to an identifier nothing resolves any
// more - 3,993 of them for one SELENA scan. They are invisible, they are the
// bulk of what an ontology weighs, and they carry object paths: on a
// regulated bucket a path is a patient identifier, so "delete this ontology"
// has to mean the paths go too.
func TestDeletingAnOntologyTakesItsFileList(t *testing.T) {
	store := NewOntologyObjectStore()
	if err := store.Create(ontology.Ontology{ID: "o1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.ReplaceObjects("o1", []ontology.Object{
		{Path: "SELENA/SELENA-01-001/20260218/IRM/scan.dcm", SubjectID: "SELENA-01-001"},
		{Path: "SELENA/SELENA-01-005/20260422/IRM/scan.dcm", SubjectID: "SELENA-01-005"},
	}); err != nil {
		t.Fatalf("replace objects: %v", err)
	}

	if err := store.Delete("o1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	count, err := store.CountObjects("o1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d object row(s) survived the ontology", count)
	}
}
