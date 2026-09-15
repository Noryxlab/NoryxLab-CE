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
// did not unblock her - attaching regulated data to a project is a global
// administrator's decision, which ownership does not confer - and it removed
// the access every other member of that organization held through it.
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
