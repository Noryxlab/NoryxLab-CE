package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// Are the files an extract froze still there?
//
// Freezing a selection protects the *n*: the same extract names the same files
// next month, whatever the study recruits. It does not protect the bytes.
// Objects get deleted, buckets get pruned, a prefix gets re-filed - and a
// frozen list pointing at objects that no longer exist mounts as a tree of
// dangling symlinks, which a notebook discovers one `open()` at a time.
//
// An ontology's freshness answers a different question - has the source grown -
// and it is the wrong one here. An extract does not care that the study
// recruited eleven more subjects; it cares whether its own files survive.
//
// Checked on a sample, and the answer says so. Verifying four thousand objects
// means four thousand round trips to Cellar, which is a minute of somebody's
// afternoon to answer a question that is almost always "yes". A sample of
// twenty-five spread across the list turns "everything is fine" into a second
// and still catches a bucket that has been pruned, which is the failure that
// actually happens.

const (
	extractIntegritySample  = 25
	extractIntegrityTimeout = 20 * time.Second
)

type extractIntegrityMissing struct {
	Path string `json:"path"`
}

// GetExtractIntegrity reports how much of a sample of the frozen list is still
// in the bucket.
func (h Handlers) GetExtractIntegrity(w http.ResponseWriter, r *http.Request) {
	_, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	ontology, found, err := h.ontologyStore.GetByID(item.OntologyID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "the ontology this extract was drawn from is gone"})
		return
	}
	source, found, err := h.datasetStore.GetByID(ontology.SourceID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "the dataset this extract reads is gone"})
		return
	}
	client, _, err := h.datasetS3Client(source)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}

	members, err := h.extractStore.ListMembers(item.ID, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the frozen list"})
		return
	}
	if len(members) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"checked": 0, "missing": 0, "total": item.ObjectCount, "complete": true,
		})
		return
	}

	prefix := strings.TrimPrefix(strings.TrimSpace(source.Prefix), "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	ctx, cancel := contextWithTimeoutFrom(r, extractIntegrityTimeout)
	defer cancel()

	// Spread across the list rather than the first twenty-five. A bucket is
	// pruned by date or by subject, so the beginning of an alphabetical list is
	// precisely where nothing is missing.
	step := len(members) / extractIntegritySample
	if step < 1 {
		step = 1
	}
	checked := 0
	missing := []extractIntegrityMissing{}
	for index := 0; index < len(members) && checked < extractIntegritySample; index += step {
		member := members[index]
		if _, err := client.StatObject(ctx, source.Bucket, prefix+member.Path, minio.StatObjectOptions{}); err != nil {
			if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
				missing = append(missing, extractIntegrityMissing{Path: member.Path})
				checked++
				continue
			}
			// A storage error is not a missing file, and reporting it as one
			// would tell somebody their study has been deleted because Cellar
			// was slow.
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the source could not be read: " + err.Error()})
			return
		}
		checked++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"checked":      checked,
		"missing":      len(missing),
		"missingPaths": missing,
		"total":        item.ObjectCount,
		// Complete is about the sample, and the field names say so: a caller
		// that treats "complete" as a guarantee over four thousand files would
		// be reading more than was measured.
		"complete": len(missing) == 0,
	})
}
