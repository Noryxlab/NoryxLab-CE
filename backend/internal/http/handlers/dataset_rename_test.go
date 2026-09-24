package handlers

import (
	"net/http"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

// A rename cannot be used to reach outside the dataset.
//
// Both sides go through the same cleaning as every other object path. Without
// it, naming "../" as the target would write into a bucket prefix the caller
// was never granted - and a rename is the one operation that takes two paths,
// so it is the one where half a check is easy to leave behind.
func TestRenameStaysInsideTheDataset(t *testing.T) {
	item := dataset.Dataset{Bucket: "b", Prefix: "etude-a"}
	for _, raw := range []string{"../ailleurs/fichier.dcm", "a/../../b.dcm", "/../../etc/passwd"} {
		rel, key := datasetObjectKey(item, raw)
		if rel == "" {
			continue // refused outright by the handler
		}
		if got := key; got[:len("etude-a/")] != "etude-a/" {
			t.Fatalf("%q escaped the prefix: %q", raw, got)
		}
	}
}

// An absent target and an unreadable one are not the same thing.
//
// The handler proceeds only when the target is genuinely absent. Reading any
// error as "absent" would let a rename overwrite a file whose existence could
// not be checked, which on a regulated dataset is a deletion with no record of
// what was lost.
func TestNoSuchKeyDistinguishesAbsentFromUnreadable(t *testing.T) {
	absent := minio.ErrorResponse{Code: "NoSuchKey", StatusCode: http.StatusNotFound}
	if !isNoSuchKey(absent) {
		t.Fatal("a missing object was not recognised as missing")
	}
	for _, other := range []minio.ErrorResponse{
		{Code: "AccessDenied", StatusCode: http.StatusForbidden},
		{Code: "InternalError", StatusCode: http.StatusInternalServerError},
	} {
		if isNoSuchKey(other) {
			t.Fatalf("%s was read as an absent object; a rename would overwrite", other.Code)
		}
	}
}

// The audit line names both halves.
//
// "deleted" and "created" as separate entries read afterwards as a loss and an
// unrelated arrival, which is exactly the question somebody asks months later.
func TestRenameAuditNamesBothNames(t *testing.T) {
	details := datasetRenameAuditDetails(
		dataset.Dataset{Name: "etude", Bucket: "b"}, "ancien.dcm", "nouveau.dcm", 2048)
	if details["to"] != "nouveau.dcm" {
		t.Fatalf("the new name is missing from the audit line: %v", details)
	}
	found := false
	for _, v := range details {
		if v == "ancien.dcm" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the old name is missing from the audit line: %v", details)
	}
}
