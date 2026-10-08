package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

// Direct uploads keep a bulk transfer out of the API pod. The platform only
// decides who may write which key; the bytes travel between the sender and S3.
const datasetUploadURLLifetime = 15 * time.Minute

// datasetUploadBatchMax bounds one authorisation call.
//
// A study arrives as thousands of small files - SELENA is 3,993 objects for
// 9 GB, so 2.3 MB each - and authorising them one at a time costs two API
// round trips per file: around 8,000 requests to move one study, with an
// audit entry for each. That is the "audit too granular to steer an import"
// problem, and it is solved by asking for a batch rather than by auditing
// less.
//
// Two hundred keeps the response under about 200 kB of presigned URLs, which
// a sender can hold in memory and spend inside the lifetime above.
const datasetUploadBatchMax = 200

type datasetUploadURLRequest struct {
	Path string `json:"path"`
	// Size is what the sender is about to write, declared so the platform can
	// apply its own ceiling.
	//
	// The proxied upload refuses an object over maxDatasetObjectBytes and
	// holds the stream to the declared length. A presigned PUT passes through
	// none of that: the platform signs a key and never sees the bytes, so the
	// only remaining ceiling is S3's own 5 GiB single-PUT cap - the same
	// figure, by coincidence rather than by decision, and stated nowhere.
	// Declaring the size restores the control where it can still be applied.
	// A sender who lies gets whatever S3 accepts, which is exactly the
	// position the proxied path is in once it has read the declared count.
	Size int64 `json:"size,omitempty"`
}

type datasetUploadBatchRequest struct {
	Objects []datasetUploadURLRequest `json:"objects"`
}

type datasetUploadCompleteRequest struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

type datasetUploadCompleteBatchRequest struct {
	Objects []datasetUploadCompleteRequest `json:"objects"`
}

// datasetDirectUploadProblem says when a presigned URL cannot be used by the
// sender it would be handed to.
//
// A presigned URL carries the host the platform's own client talks to. For a
// dataset on the internal MinIO profile that host is a cluster service -
// minio.noryx.svc.cluster.local - which resolves inside the cluster and
// nowhere else. The platform was signing those happily and handing them to
// machines outside: curl exit 6, "could not resolve host", several hundred
// times, with nothing to connect the failure to its cause. Found on the first
// real import against the DC, which is the only way it could have been found.
//
// Refusing names the other path rather than leaving somebody to guess: the
// proxied upload works for any dataset, carries the bytes through the API,
// and is the right answer when the store is not public.
//
// Only the internal profile is refused. An external endpoint is whatever an
// operator configured, and the platform has no business second-guessing a
// host it was told to use.
func datasetDirectUploadProblem(item datasetdomain.Dataset) string {
	if strings.EqualFold(strings.TrimSpace(item.Provider), "minio") {
		return "this dataset is stored on the platform's internal object store, which is not reachable from outside the cluster; send it with PUT /api/v1/datasets/{datasetID}/objects/{path} instead"
	}
	return ""
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
	if probleme := datasetDirectUploadProblem(item); probleme != "" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": probleme, "code": "direct_upload_unavailable"})
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
	if probleme := datasetUploadSizeProblem(req); probleme != "" {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": probleme})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	url, err := client.PresignedPutObject(r.Context(), item.Bucket, key, datasetUploadURLLifetime)
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

// datasetUploadSizeProblem applies the platform's own object ceiling to a
// declared size, or says nothing when none was declared.
func datasetUploadSizeProblem(req datasetUploadURLRequest) string {
	if req.Size < 0 {
		return "a declared size cannot be negative"
	}
	if req.Size > maxDatasetObjectBytes {
		return fmt.Sprintf("%s is larger than the %d GiB an object may carry",
			req.Path, maxDatasetObjectBytes>>30)
	}
	return ""
}

// CreateDatasetUploadURLs authorises a batch in one call.
//
// Two round trips per file is the wrong shape for the job this exists for.
// A study arrives as thousands of small objects, so moving SELENA means
// around 8,000 requests and 8,000 audit entries - which is not a security
// property, it is a log nobody can read. One call authorises up to
// datasetUploadBatchMax keys and records one entry saying who, which dataset,
// how many objects and how many bytes.
//
// Partial by design. An object whose path or size is refused is reported in
// its own right and the rest are still authorised: a single bad name in a
// directory of four thousand should cost that file, not the import. The
// sender reads "refused" and decides.
func (h Handlers) CreateDatasetUploadURLs(w http.ResponseWriter, r *http.Request) {
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
	if probleme := datasetDirectUploadProblem(item); probleme != "" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": probleme, "code": "direct_upload_unavailable"})
		return
	}
	var req datasetUploadBatchRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil || len(req.Objects) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a non-empty objects array is required"})
		return
	}
	if len(req.Objects) > datasetUploadBatchMax {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("a batch authorises at most %d objects", datasetUploadBatchMax)})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}

	transferID := uuid.NewString()
	expiresAt := time.Now().UTC().Add(datasetUploadURLLifetime)
	authorised := make([]map[string]any, 0, len(req.Objects))
	refused := make([]map[string]string, 0)
	var bytes int64

	for _, object := range req.Objects {
		key, valid := datasetDirectObjectKey(item.Prefix, object.Path)
		if !valid {
			refused = append(refused, map[string]string{
				"path": object.Path, "reason": "object path must be a non-empty relative path"})
			continue
		}
		if probleme := datasetUploadSizeProblem(object); probleme != "" {
			refused = append(refused, map[string]string{"path": object.Path, "reason": probleme})
			continue
		}
		url, err := client.PresignedPutObject(r.Context(), item.Bucket, key, datasetUploadURLLifetime)
		if err != nil {
			refused = append(refused, map[string]string{
				"path": object.Path, "reason": "failed to authorize direct dataset upload"})
			continue
		}
		bytes += object.Size
		authorised = append(authorised, map[string]any{
			"path":   object.Path,
			"key":    key,
			"method": http.MethodPut,
			"url":    url.String(),
		})
	}

	// One entry for the batch: who, which dataset, how many, how much. The
	// per-object record is the confirmation, which is where the bytes that
	// actually arrived are known.
	details := datasetTransferAuditDetails(item, "", bytes)
	details["transferId"] = transferID
	details["expiresAt"] = expiresAt.Format(time.RFC3339)
	details["objects"] = len(authorised)
	details["refused"] = len(refused)
	outcome := "success"
	if len(authorised) == 0 {
		outcome = "failure"
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload_authorize", "dataset", item.ID, "", outcome, "batch", details)

	writeJSON(w, http.StatusCreated, map[string]any{
		"transferId": transferID,
		"expiresAt":  expiresAt,
		"bucket":     item.Bucket,
		"objects":    authorised,
		"refused":    refused,
	})
}

// ConfirmDatasetUploads verifies a batch of directly written objects.
//
// The authorisation side was batched and the confirmation side was not, which
// left the confirmation as the whole cost: a terabyte of DICOM slices is some
// 435,000 objects, so one call each is 435,000 round trips to close an import
// whose URLs took two thousand. Verification still happens per object -
// each one is stat-ed against its declared size, because that is the only
// check worth making - but it happens inside one request.
//
// Partial, like the authorisation: an object that is absent or the wrong size
// is reported on its own and the others are still recorded. A transfer that
// lost three files out of four hundred thousand should tell you which three.
func (h Handlers) ConfirmDatasetUploads(w http.ResponseWriter, r *http.Request) {
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
	var req datasetUploadCompleteBatchRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil || len(req.Objects) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a non-empty objects array is required"})
		return
	}
	if len(req.Objects) > datasetUploadBatchMax {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("a batch confirms at most %d objects", datasetUploadBatchMax)})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}

	verified := make([]map[string]any, 0, len(req.Objects))
	failed := make([]map[string]string, 0)
	var bytes int64

	for _, object := range req.Objects {
		key, valid := datasetDirectObjectKey(item.Prefix, object.Path)
		if !valid || object.Size < 0 {
			failed = append(failed, map[string]string{
				"path": object.Path, "reason": "path and non-negative size are required"})
			continue
		}
		info, err := client.StatObject(r.Context(), item.Bucket, key, minio.StatObjectOptions{})
		if err != nil || info.Size != object.Size {
			failed = append(failed, map[string]string{
				"path": object.Path, "reason": "the uploaded object is absent or has an unexpected size"})
			continue
		}
		bytes += info.Size
		verified = append(verified, map[string]any{"path": object.Path, "key": key, "size": info.Size})
	}

	// One entry for the batch, and the checksums with it: the sender's
	// manifest value is what makes an import auditable after the fact, and
	// dropping it to save a column would be saving the wrong thing.
	details := datasetTransferAuditDetails(item, "", bytes)
	details["objects"] = len(verified)
	details["failed"] = len(failed)
	details["checksums"] = datasetUploadChecksums(req.Objects)
	outcome := "success"
	reason := "batch"
	if len(verified) == 0 {
		outcome = "failure"
		reason = "verification_failed"
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", outcome, reason, details)

	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":   item.Bucket,
		"verified": verified,
		"failed":   failed,
	})
}

// datasetUploadChecksums keeps the sender's SHA-256 values beside their paths.
func datasetUploadChecksums(objects []datasetUploadCompleteRequest) map[string]string {
	out := map[string]string{}
	for _, object := range objects {
		if checksum := strings.ToLower(strings.TrimSpace(object.SHA256)); checksum != "" {
			out[object.Path] = checksum
		}
	}
	return out
}
