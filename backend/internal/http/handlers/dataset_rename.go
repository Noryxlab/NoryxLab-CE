package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
		// Not a single object: it may be a folder, which is a prefix rather
		// than a thing. Only then, so a plain rename keeps its exact
		// behaviour and its one-object audit line.
		if !isNoSuchKey(err) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the object to rename could not be read"})
			return
		}
		count, moveErr := h.renameDatasetFolder(ctx, client, item, fromKey, toKey)
		switch {
		case errors.Is(moveErr, errNothingToMove):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "the object to rename does not exist"})
		case moveErr != nil:
			h.emitAdvancedAudit(r, identity.UserID(), "dataset.folder.renamed", "dataset", item.ID, "",
				"failure", "move_failed", datasetRenameAuditDetails(item, fromRel, toRel, 0))
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": moveErr.Error()})
		default:
			h.emitAdvancedAudit(r, identity.UserID(), "dataset.folder.renamed", "dataset", item.ID, "",
				"success", "", datasetRenameAuditDetails(item, fromRel, toRel, int64(count)))
			writeJSON(w, http.StatusOK, map[string]any{"from": fromRel, "to": toRel, "objects": count})
		}
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

// How many objects one move may carry.
//
// A folder move is one HTTP call that does N server-side copies, and the
// handler holds a ten-minute budget. A patient folder is hundreds of files
// and a study is thousands, which fits; the root of a 381 GB dataset does
// not, and the honest failure is to say so with the count before starting
// rather than to time out halfway and leave the dataset in two states.
const maxObjectsPerFolderMove = 10000

// renameDatasetFolder moves everything under one prefix to another.
//
// The rename handler above stats a single object, so a folder - which is a
// prefix, not a thing - answered "the object to rename does not exist". The
// interface offered the button anyway, on any single selection, so renaming
// a folder was an affordance that always failed.
//
// The order is what matters: every object is copied and each copy verified by
// size before a single source is removed. A move that fails halfway therefore
// leaves the originals untouched, and the copies it had already made are
// swept away. The dataset is never in a state where a file exists under
// neither name - which, on study data somebody is reorganising, is the only
// property worth protecting.
func (h Handlers) renameDatasetFolder(
	ctx context.Context, client *minio.Client, item dataset.Dataset, fromKey, toKey string,
) (int, error) {
	fromPrefix, toPrefix := fromKey+"/", toKey+"/"
	if strings.HasPrefix(toPrefix, fromPrefix) {
		return 0, errors.New("a folder cannot be moved inside itself")
	}

	sources := []minio.ObjectInfo{}
	for object := range client.ListObjects(ctx, item.Bucket,
		minio.ListObjectsOptions{Prefix: fromPrefix, Recursive: true}) {
		if object.Err != nil {
			return 0, errors.New("the folder could not be listed")
		}
		sources = append(sources, object)
		if len(sources) > maxObjectsPerFolderMove {
			return 0, errors.New("this folder holds more than " +
				itoa(maxObjectsPerFolderMove) + " files; move its sub-folders one at a time")
		}
	}
	if len(sources) == 0 {
		return 0, errNothingToMove
	}

	// Refused before anything is copied, and for the whole move: a folder that
	// merged itself into an existing one, file by file, would be impossible to
	// undo and impossible to describe afterwards.
	for _, source := range sources {
		target := toPrefix + strings.TrimPrefix(source.Key, fromPrefix)
		if _, err := client.StatObject(ctx, item.Bucket, target, minio.StatObjectOptions{}); err == nil {
			return 0, errors.New("the destination already holds " +
				strings.TrimPrefix(source.Key, fromPrefix))
		} else if !isNoSuchKey(err) {
			return 0, errors.New("the destination could not be checked")
		}
	}

	copied := []string{}
	for _, source := range sources {
		target := toPrefix + strings.TrimPrefix(source.Key, fromPrefix)
		if _, err := client.CopyObject(ctx,
			minio.CopyDestOptions{Bucket: item.Bucket, Object: target},
			minio.CopySrcOptions{Bucket: item.Bucket, Object: source.Key}); err != nil {
			h.sweepCopies(ctx, client, item, copied)
			return 0, errors.New("the copy failed at " +
				strings.TrimPrefix(source.Key, fromPrefix) + "; nothing was moved")
		}
		// Verified by size, for the same reason as a single object: a copy
		// that reported success and landed short must not be followed by the
		// deletion of the only complete one.
		landed, err := client.StatObject(ctx, item.Bucket, target, minio.StatObjectOptions{})
		if err != nil || landed.Size != source.Size {
			h.sweepCopies(ctx, client, item, append(copied, target))
			return 0, errors.New("the copy of " +
				strings.TrimPrefix(source.Key, fromPrefix) + " could not be verified; nothing was moved")
		}
		copied = append(copied, target)
	}

	// Every copy stands. Only now do the originals go.
	for _, source := range sources {
		if err := client.RemoveObject(ctx, item.Bucket, source.Key, minio.RemoveObjectOptions{}); err != nil {
			return len(sources), errors.New("the folder was copied but " +
				strings.TrimPrefix(source.Key, fromPrefix) + " could not be removed; both names now exist")
		}
	}
	// The folder marker, when the store keeps one. Absent on most, and its
	// absence is not a failure.
	_ = client.RemoveObject(ctx, item.Bucket, fromPrefix, minio.RemoveObjectOptions{})
	return len(sources), nil
}

// errNothingToMove separates "this prefix holds nothing" from a real failure,
// so the handler can answer "does not exist" as it always did.
var errNothingToMove = errors.New("no object under this prefix")

// sweepCopies removes what a failed move had already written. Best effort: the
// move has failed either way, and the originals are what matters.
func (h Handlers) sweepCopies(ctx context.Context, client *minio.Client, item dataset.Dataset, keys []string) {
	for _, key := range keys {
		_ = client.RemoveObject(ctx, item.Bucket, key, minio.RemoveObjectOptions{})
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
