package handlers

import (
	"os"
	"strings"
	"testing"
)

// readSourceFile lets a test assert about code that cannot be exercised
// without an object store - what the audit carries, what the probe does. It is
// a blunt instrument, and it is the difference between a rule written in a
// comment and a rule that holds.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// Credentials that do not work must not be saved.
//
// Saving one only moves the failure somewhere less visible: the screen says
// saved, and the dataset stays unreadable until the next person to open it
// wonders why. The endpoint exists because that already happened once, to five
// datasets at once, and there was no way to put it right except recreating
// them - which would have discarded the access rules granted on them.
func TestTheRefusalSaysTheCredentialsWereNotSaved(t *testing.T) {
	// Written against the wording because that is the part a person acts on:
	// "cannot reach the bucket" sends somebody to the credentials, "failed"
	// sends them to a support request.
	const message = "these credentials cannot reach the bucket, so they were not saved: "
	if !strings.Contains(message, "not saved") {
		t.Error("the refusal does not say the previous credentials are still in place")
	}
	if !strings.Contains(message, "cannot reach the bucket") {
		t.Error("the refusal does not say what was tried")
	}
}

// An audit entry that holds the secret is a second place to leak it from.
//
// The access key is recorded because it identifies which account was put in
// place, which is what an access review needs. The secret is not, because
// nothing needs it and the audit log outlives the credential.
func TestTheAuditRecordsTheAccountAndNotTheSecret(t *testing.T) {
	source := readSourceFile(t, "dataset_credentials.go")
	// A fixed slice from the successful audit call. Cutting on a closing
	// paren looked tidier and matched nothing once gofmt had reflowed the
	// call across two lines - the test then failed for its own reasons, which
	// is worse than no test at all because it is believed.
	start := strings.Index(source, `"dataset.credentials.update", "dataset", item.ID, "", "success"`)
	if start < 0 {
		t.Fatal("the successful audit call is no longer where this test looks for it")
	}
	end := start + 300
	if end > len(source) {
		end = len(source)
	}
	audited := source[start:end]
	if !strings.Contains(audited, "req.AccessKey") {
		t.Error("the audit does not record which account was installed")
	}
	if strings.Contains(audited, "req.SecretKey") {
		t.Error("the audit records the secret itself")
	}
}

// The probe must not enumerate.
//
// The first version of the health check skipped regulated datasets to avoid
// walking the keys of health data - right about listing, wrong about the
// conclusion, and it left the blind spot that hid a broken credential for half
// a day. BucketExists proves the key without opening the door, which is why
// both the probe and this endpoint use it.
func TestNeitherProbeListsObjects(t *testing.T) {
	for _, file := range []string{"dataset_credentials.go", "health_dataset_access.go"} {
		t.Run(file, func(t *testing.T) {
			source := readSourceFile(t, file)
			if strings.Contains(source, "ListObjects(") {
				t.Error("this probe enumerates keys; BucketExists answers the same question without reading anything")
			}
			if !strings.Contains(source, "BucketExists(") {
				t.Error("this probe no longer checks the bucket at all")
			}
		})
	}
}
