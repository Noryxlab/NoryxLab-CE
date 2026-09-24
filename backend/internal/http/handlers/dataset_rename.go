package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

// Renaming an object inside a dataset.
//
// Asked for by a customer working on a regulated dataset, and it looked at
// first like a request for more write capability on HDS data - which the
// platform is meant to be moving away from. It is not. A writer can already
// rename a file today: download it, PUT it under the new name, DELETE the old
// one. Every one of those three calls is already permitted by the same
// canWriteDataset check this handler uses.
//
// So this adds no privilege. What it adds is atomicity and a single audit
// line, and it removes the window in which the manual sequence leaves the
// object duplicated or absent. It is the safer way to do something already
// possible, not a new power.
//
// The copy is server-side, so a two gigabyte study moves without its bytes
// crossing this process, and the delete does not run until the copy has been
// confirmed to exist. That ordering is the whole design: the failure to avoid
// is not a failed rename, it is a delete that succeeded after a copy that did
// not.

type renameDatasetObjectRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (h Handlers) RenameDatasetObject(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canWriteDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}

	var req renameDatasetObjectRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	// Both sides go through the same cleaning as every other object path, so a
	// rename cannot reach out of the dataset by naming "../" as its target -
	// which would otherwise be a way to write into a bucket the caller was
	// never granted.
	fromRel, fromKey := datasetObjectKey(item, req.From)
	toRel, toKey := datasetObjectKey(item, req.To)
	switch {
	case fromRel == "" || fromRel == ".":
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the object to rename is required"})
		return
	case toRel == "" || toRel == ".":
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the new name is required"})
		return
	case fromRel == toRel:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the new name is the same as the old one"})
		return
	}

	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	source, err := client.StatObject(ctx, item.Bucket, fromKey, minio.StatObjectOptions{})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "the object to rename does not exist"})
		return
	}

	// Refused rather than overwritten. A rename that silently replaces another
	// file destroys something nobody asked about, and on a regulated dataset
	// that is a deletion with no record of what was lost.
	if _, err := client.StatObject(ctx, item.Bucket, toKey, minio.StatObjectOptions{}); err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "an object with that name already exists"})
		return
	} else if !isNoSuchKey(err) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to check the target name"})
		return
	}

	if _, err := client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: item.Bucket, Object: toKey},
		minio.CopySrcOptions{Bucket: item.Bucket, Object: fromKey}); err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.renamed", "dataset", item.ID, "",
			"failure", "copy_failed", datasetRenameAuditDetails(item, fromRel, toRel, source.Size))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the copy failed, nothing was changed"})
		return
	}

	// Confirmed before anything is destroyed, and confirmed by size rather
	// than by the absence of an error: a copy that reported success and landed
	// short would otherwise be followed by the deletion of the only complete
	// copy.
	copied, err := client.StatObject(ctx, item.Bucket, toKey, minio.StatObjectOptions{})
	if err != nil || copied.Size != source.Size {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.renamed", "dataset", item.ID, "",
			"failure", "copy_unverified", datasetRenameAuditDetails(item, fromRel, toRel, source.Size))
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "the copy could not be verified; the original was left in place"})
		return
	}

	if err := client.RemoveObject(ctx, item.Bucket, fromKey, minio.RemoveObjectOptions{}); err != nil {
		// The copy stands and the original stands: the caller now has two
		// files rather than none, which is the right way round to fail, and
		// the audit line says so rather than leaving somebody to find it.
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.renamed", "dataset", item.ID, "",
			"failure", "source_not_removed", datasetRenameAuditDetails(item, fromRel, toRel, source.Size))
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "the copy succeeded but the original could not be removed; both names now exist"})
		return
	}

	// One line, naming both halves. "deleted" and "created" as separate
	// entries would read afterwards as a loss and an unrelated arrival.
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.renamed", "dataset", item.ID, "",
		"success", "", datasetRenameAuditDetails(item, fromRel, toRel, source.Size))
	writeJSON(w, http.StatusOK, map[string]any{
		"from": fromRel, "to": toRel, "size": source.Size,
	})
}

// isNoSuchKey says whether the target is genuinely absent, as opposed to
// unreadable. The difference decides whether a rename may proceed.
func isNoSuchKey(err error) bool {
	var response minio.ErrorResponse
	if errors.As(err, &response) {
		return response.Code == "NoSuchKey" || response.StatusCode == http.StatusNotFound
	}
	return minio.ToErrorResponse(err).Code == "NoSuchKey"
}

func datasetRenameAuditDetails(item dataset.Dataset, from, to string, size int64) map[string]any {
	details := datasetTransferAuditDetails(item, from, size)
	details["to"] = to
	return details
}
