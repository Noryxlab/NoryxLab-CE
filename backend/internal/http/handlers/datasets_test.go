package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

func TestHDSS3ClientHasNoFallback(t *testing.T) {
	h := Handlers{minioClient: &minio.Client{}}
	item := dataset.New("admin", "health", "", "health-bucket", "", "s3", "hds", "https://hds.example.com", "custom")

	client, _, err := h.datasetS3Client(item)
	if err == nil || client != nil {
		t.Fatal("expected HDS dataset to reject fallback to internal MinIO")
	}
}

func TestHDSDatasetIsUnavailableInCE(t *testing.T) {
	h := Handlers{}
	item := dataset.New("admin", "health", "", "health-bucket", "", "s3", "hds", "https://hds.example.com", "custom")

	if h.datasetAvailableInEdition(item) {
		t.Fatal("expected HDS dataset to be unavailable with CE hooks")
	}
}

func TestExternalS3ClientHasNoSharedProfileFallback(t *testing.T) {
	h := Handlers{}
	item := dataset.New("admin", "health", "", "health-bucket", "", "s3", "hds", "https://standard.example.com", "custom")

	client, _, err := h.datasetS3Client(item)
	if err == nil || client != nil {
		t.Fatal("expected external dataset without dedicated credentials to be rejected")
	}
}

// On Community, an HDS refusal really is about the edition, and the message
// says so. The Enterprise case - where it is not - is asserted in the overlay.
func TestAnHDSRefusalOnCommunityNamesTheEdition(t *testing.T) {
	h := Handlers{}
	item := dataset.New("admin", "health", "", "health-bucket", "", "s3", "hds", "https://hds.example.com", "custom")

	if message := h.datasetAssignmentError(item); message != "HDS dataset management requires NoryxLab Enterprise Edition" {
		t.Errorf("Community should name the edition, got %q", message)
	}
}

func TestAnOrdinaryRefusalNamesTheRoleAndNotTheEdition(t *testing.T) {
	h := Handlers{}
	item := dataset.New("someone", "figures", "", "figures-bucket", "", "s3", "non-hds", "https://s3.example.com", "custom")

	message := h.datasetAssignmentError(item)
	if message != "dataset owner or global admin role required to assign this dataset" {
		t.Errorf("a non-HDS refusal should name the role, got %q", message)
	}
}

// A health dataset belongs to an organization, never to a person.
//
// Both were accepted until a real one taught the difference: a dataset owned
// by Essilor was handed to one of its members trying to unblock herself. It
// did not unblock her - a mount asks for entitlement through the organisation,
// which personal ownership takes away rather than confers - and it removed the
// access every other member of that organization held through it.
func TestRegulatedDataCannotBeOwnedByAPerson(t *testing.T) {
	if ownerAllowedForClassification("hds", "user") {
		t.Error("a health dataset was handed to a person")
	}
	if !ownerAllowedForClassification("hds", "organization") {
		t.Error("an organization was refused its own health dataset")
	}
	// Case and spacing are how these values arrive from a form, not a reason
	// for the rule to stop applying.
	if ownerAllowedForClassification(" HDS ", " User ") {
		t.Error("the rule was escaped by spacing")
	}

	// Ordinary data keeps belonging to whoever registered it.
	if !ownerAllowedForClassification("non-hds", "user") {
		t.Error("an ordinary dataset was refused a personal owner")
	}
	if !ownerAllowedForClassification("", "user") {
		t.Error("an unclassified dataset was refused a personal owner")
	}
}

// Who may mount regulated data into a project.
//
// It was a global administrator and nobody else, which is defensible on paper
// and a queue in practice: a team whose organisation owns the data, working in
// a project they administer, waited on one person for a mount involving nobody
// outside their own organisation. The entitlement is what the wait stood in
// for, so the entitlement is what is asked.
//
// Only this half is exercised here: the other half asks the directory who
// administers a project, and the directory is not something this test can
// stand up honestly.
func TestRegulatedDataIsMountedOnlyByItsOwnOrganisation(t *testing.T) {
	item := dataset.New("essilor", "cohort", "", "hds-bucket", "", "s3", "hds", "https://hds.example.com", "custom")
	item.OwnerType = "organization"
	item.OwnerID = "essilor"

	member := []dataset.Subject{{Type: "user", ID: "claire"}, {Type: "organization", ID: "essilor"}}
	if !entitledToRegulatedDataset(member, item) {
		t.Error("a member of the owning organisation was refused")
	}

	// The case a real transfer produced: the dataset was handed to a person,
	// which unblocked nobody and cut off everyone who had it through the
	// organisation.
	personal := item
	personal.OwnerType = "user"
	personal.OwnerID = "claire"
	if entitledToRegulatedDataset(member, personal) {
		t.Error("regulated data owned by a person opened a mount")
	}

	outsider := []dataset.Subject{{Type: "user", ID: "someone"}, {Type: "organization", ID: "inria"}}
	if entitledToRegulatedDataset(outsider, item) {
		t.Error("another organisation could mount data it does not own")
	}

	// A user whose identifier happens to match the organisation's is not the
	// organisation.
	impostor := []dataset.Subject{{Type: "user", ID: "essilor"}}
	if entitledToRegulatedDataset(impostor, item) {
		t.Error("a user identifier was read as an organisation")
	}

	// Case and spacing are how these values arrive from a form and from the
	// directory, not a reason for the rule to stop applying.
	spaced := []dataset.Subject{{Type: " Organization ", ID: " Essilor "}}
	if !entitledToRegulatedDataset(spaced, item) {
		t.Error("the rule was defeated by spacing")
	}
}
