package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

// Declaring how a dataset's paths are read, by hand or from a proposal.
//
// The platform had one rule compiled in, and the second study to arrive
// numbered its patients differently. The assistant could already diagnose that
// from path shapes and say which position held what - and there was nowhere to
// put the answer, so the conversation ended in a panel (ADR-040).
//
// What is missing is not intelligence, it is a place to write the answer down.
// These three endpoints are that place: read the rule, try it, store it. The
// assistant becomes an accelerator for filling three fields, and an
// installation with no assistant configured fills them itself.

type pathLayoutRequest struct {
	SubjectLevel  *int `json:"subjectLevel"`
	VisitLevel    *int `json:"visitLevel"`
	ModalityLevel *int `json:"modalityLevel"`
}

// layoutFrom reads the request, treating an absent level as "not in the path"
// rather than as level zero.
func layoutFrom(req pathLayoutRequest) datasetdomain.PathLayout {
	level := func(value *int) int {
		if value == nil {
			return datasetdomain.LevelAbsent
		}
		return *value
	}
	return datasetdomain.PathLayout{
		SubjectLevel:  level(req.SubjectLevel),
		VisitLevel:    level(req.VisitLevel),
		ModalityLevel: level(req.ModalityLevel),
	}
}

// requireDatasetOwner answers 404 itself, and refuses anybody who may read the
// dataset but not administer it: the layout decides how every future scan
// reads this bucket, which is the owner's call.
func (h Handlers) requireDatasetForLayout(w http.ResponseWriter, r *http.Request) (datasetdomain.Dataset, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return datasetdomain.Dataset{}, false
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return datasetdomain.Dataset{}, false
	}
	if !h.isGlobalAdmin(identity) && h.datasetRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return datasetdomain.Dataset{}, false
	}
	return item, true
}

// GetDatasetPathLayout returns the rule this dataset is read with.
func (h Handlers) GetDatasetPathLayout(w http.ResponseWriter, r *http.Request) {
	item, ok := h.requireDatasetForLayout(w, r)
	if !ok {
		return
	}
	payload := map[string]any{"declared": false, "description": "no layout declared"}
	if item.PathLayout != nil && item.PathLayout.Declared() {
		payload = map[string]any{
			"declared":      true,
			"subjectLevel":  item.PathLayout.SubjectLevel,
			"visitLevel":    item.PathLayout.VisitLevel,
			"modalityLevel": item.PathLayout.ModalityLevel,
			"description":   item.PathLayout.Describe(),
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

// SetDatasetPathLayout stores it, or clears it with an empty body.
func (h Handlers) SetDatasetPathLayout(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, ok := h.requireDatasetForLayout(w, r)
	if !ok {
		return
	}
	if !h.canManageDatasetAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "dataset owner or global admin required"})
		return
	}
	var req pathLayoutRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid layout"})
			return
		}
	}
	// Nothing named means "forget the rule and go back to the compiled one",
	// which is the way out when a declared layout turns out to be wrong.
	if req.SubjectLevel == nil {
		if err := h.datasetStore.SetPathLayout(item.ID, nil); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear the layout"})
			return
		}
		h.emitAudit(r, identity.UserID(), "dataset.path_layout.cleared", "dataset", item.ID, "", "success", "", nil)
		writeJSON(w, http.StatusOK, map[string]any{"declared": false, "description": "no layout declared"})
		return
	}

	layout := layoutFrom(req)
	if probleme := layout.Validate(); probleme != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": probleme})
		return
	}
	if err := h.datasetStore.SetPathLayout(item.ID, &layout); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the layout"})
		return
	}
	h.emitAudit(r, identity.UserID(), "dataset.path_layout.changed", "dataset", item.ID, "", "success", "",
		map[string]any{"layout": layout.Describe()})
	writeJSON(w, http.StatusOK, map[string]any{
		"declared":      true,
		"subjectLevel":  layout.SubjectLevel,
		"visitLevel":    layout.VisitLevel,
		"modalityLevel": layout.ModalityLevel,
		"description":   layout.Describe(),
	})
}

// pathLayoutTrial is what a rule would read, on this dataset's real paths.
type pathLayoutTrial struct {
	Path     string `json:"path"`
	Subject  string `json:"subject"`
	Visit    string `json:"visit"`
	Modality string `json:"modality"`
}

const (
	pathLayoutTrialSize = 12
	// A trial answers a form field: it reads enough keys to be convincing and
	// stops, where a scan reads the bucket.
	pathLayoutTrialTimeout     = 20 * time.Second
	pathLayoutTrialScanCeiling = 2000
)

// TryDatasetPathLayout applies a candidate rule to real keys and says what it
// would read, without storing anything.
//
// This is what makes the rule writable by hand. Three numbers are abstract;
// "level 1 reads SELENA-01-001 on this path" is checkable in a second by the
// person who knows the study, and wrong in a way they will see immediately.
//
// The sample is real paths, returned only to somebody who may already read the
// bucket, and it never goes near a model: on these datasets a key is a patient
// identifier, which is why the assistant is shown shapes instead (ADR-040).
func (h Handlers) TryDatasetPathLayout(w http.ResponseWriter, r *http.Request) {
	item, ok := h.requireDatasetForLayout(w, r)
	if !ok {
		return
	}
	var req pathLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid layout"})
		return
	}
	layout := layoutFrom(req)
	if probleme := layout.Validate(); probleme != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": probleme})
		return
	}

	paths, err := h.sampleDatasetPaths(r, item, pathLayoutTrialSize)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list the dataset: " + err.Error()})
		return
	}
	trials := make([]pathLayoutTrial, 0, len(paths))
	recognised := 0
	for _, path := range paths {
		subject, visit, modality := layout.Read(path)
		if subject != "" {
			recognised++
		}
		trials = append(trials, pathLayoutTrial{Path: path, Subject: subject, Visit: visit, Modality: modality})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       trials,
		"recognised":  recognised,
		"sampled":     len(trials),
		"description": layout.Describe(),
	})
}

// sampleDatasetPaths lists a handful of real keys, deepest-looking first.
//
// A sample of the shallowest keys would be the study directory and nothing
// else, and a rule tried against those would look wrong when it is right. The
// listing stops as soon as it has enough: this answers a form field, it is not
// a scan.
func (h Handlers) sampleDatasetPaths(r *http.Request, item datasetdomain.Dataset, wanted int) ([]string, error) {
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		return nil, errDatasetUnreachable(err)
	}
	prefix := strings.TrimPrefix(strings.TrimSpace(item.Prefix), "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	ctx, cancel := contextWithTimeoutFrom(r, pathLayoutTrialTimeout)
	defer cancel()

	paths := make([]string, 0, wanted)
	examined := 0
	for obj := range client.ListObjects(ctx, item.Bucket, minioListRecursive(prefix)) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		examined++
		relPath := strings.Trim(strings.TrimPrefix(obj.Key, prefix), "/")
		// A zero-byte key that names a directory is not a path anybody reads,
		// and showing one in a trial would suggest the rule failed on a file.
		if relPath == "" || (obj.Size == 0 && !strings.Contains(relPath, ".")) {
			continue
		}
		paths = append(paths, relPath)
		if len(paths) >= wanted || examined > pathLayoutTrialScanCeiling {
			break
		}
	}
	return paths, nil
}

func minioListRecursive(prefix string) minio.ListObjectsOptions {
	return minio.ListObjectsOptions{Prefix: prefix, Recursive: true}
}

func contextWithTimeoutFrom(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func errDatasetUnreachable(err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("no object storage is configured for this dataset")
}
