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
