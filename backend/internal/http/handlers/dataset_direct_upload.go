package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// Direct uploads keep a bulk transfer out of the API pod. The platform only
// decides who may write which key; the bytes travel between the sender and S3.
const datasetUploadURLLifetime = 15 * time.Minute

type datasetUploadURLRequest struct {
	Path string `json:"path"`
}

type datasetUploadCompleteRequest struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

func datasetDirectObjectKey(itemPrefix, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return "", false
	}
	clean := strings.TrimPrefix(path.Clean("/"+raw), "/")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	if prefix := strings.Trim(itemPrefix, "/"); prefix != "" {
		return prefix + "/" + clean, true
	}
	return clean, true
}

// CreateDatasetUploadURL authorizes one short-lived direct S3 PUT. It never
// returns provider credentials and it does not proxy health-data bytes.
func (h Handlers) CreateDatasetUploadURL(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || !h.canWriteDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	var req datasetUploadURLRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a JSON object path is required"})
		return
	}
	key, valid := datasetDirectObjectKey(item.Prefix, req.Path)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path must be a non-empty relative path"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	url, err := client.PresignedPutObject(context.Background(), item.Bucket, key, datasetUploadURLLifetime)
	if err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload_authorize", "dataset", item.ID, "", "failure", "presign_failed", datasetTransferAuditDetails(item, req.Path, 0))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to authorize direct dataset upload"})
		return
	}
	transferID := uuid.NewString()
	expiresAt := time.Now().UTC().Add(datasetUploadURLLifetime)
	details := datasetTransferAuditDetails(item, req.Path, 0)
	details["transferId"] = transferID
	details["expiresAt"] = expiresAt.Format(time.RFC3339)
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload_authorize", "dataset", item.ID, "", "success", "", details)
	writeJSON(w, http.StatusCreated, map[string]any{
		"transferId": transferID,
		"method":     http.MethodPut,
		"url":        url.String(),
		"expiresAt":  expiresAt,
		"bucket":     item.Bucket,
		"key":        key,
	})
}

// ConfirmDatasetUpload verifies that the directly written object exists at the
// expected size before recording the completed transfer. SHA-256 is retained
// as the sender's manifest checksum; S3 ETags are not portable SHA-256 values.
func (h Handlers) ConfirmDatasetUpload(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || !h.canWriteDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	var req datasetUploadCompleteRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req) != nil || req.Size < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path and non-negative size are required"})
		return
	}
	key, valid := datasetDirectObjectKey(item.Prefix, req.Path)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path must be a non-empty relative path"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	info, err := client.StatObject(r.Context(), item.Bucket, key, minio.StatObjectOptions{})
	if err != nil || info.Size != req.Size {
		details := datasetTransferAuditDetails(item, req.Path, req.Size)
		details["actualSize"] = info.Size
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "failure", "verification_failed", details)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the uploaded object is absent or has an unexpected size"})
		return
	}
	details := datasetTransferAuditDetails(item, req.Path, info.Size)
	if checksum := strings.ToLower(strings.TrimSpace(req.SHA256)); checksum != "" {
		details["sha256"] = checksum
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "success", "direct", details)
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "bucket": item.Bucket, "key": key, "size": info.Size})
}
