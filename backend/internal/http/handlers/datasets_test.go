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

func TestExternalS3ClientHasNoSharedProfileFallback(t *testing.T) {
	h := Handlers{}
	item := dataset.New("admin", "health", "", "health-bucket", "", "s3", "hds", "https://standard.example.com", "custom")

	client, _, err := h.datasetS3Client(item)
	if err == nil || client != nil {
		t.Fatal("expected external dataset without dedicated credentials to be rejected")
	}
}
