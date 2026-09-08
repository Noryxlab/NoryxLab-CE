package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

// Whether an ontology still describes its source.
//
// An ontology is a photograph, and it was presented as a fact. The one on the
// DC dated from June and said "18,738 objects, 20 subjects" with exactly the
// same confidence as the one scanned this morning, which says 24,179 and 31 -
// the study had gained eleven subjects and nothing on the screen suggested the
// model had not noticed.
//
// Everything downstream rests on this: a cohort built from a stale ontology
// silently omits the subjects recruited since. So the platform counts what is
// in the source now and says how far it has drifted.
//
// The count is a listing and nothing else. These are regulated datasets; the
// platform reads their object names to say how many there are, and writes
// nothing, ever.

type ontologyFreshness struct {
	GeneratedAt     time.Time `json:"generatedAt"`
	AgeDays         int       `json:"ageDays"`
	ManifestObjects int       `json:"manifestObjects"`
	SourceObjects   int       `json:"sourceObjects"`
	Drift           int       `json:"drift"`
	Stale           bool      `json:"stale"`
	CheckedAt       time.Time `json:"checkedAt"`
}

// Counting 24,000 objects takes seconds, and a screen that renders a list of
// ontologies must not list every bucket behind them. The answer is cached for
// long enough to make repeated views free and short enough to stay true.
const ontologyFreshnessTTL = 10 * time.Minute

var ontologyFreshnessCache = struct {
	sync.Mutex
	entries map[string]ontologyFreshness
}{entries: map[string]ontologyFreshness{}}

func (h Handlers) GetOntologyFreshness(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.isGlobalAdmin(identity) && h.ontologyRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}

	ontologyFreshnessCache.Lock()
	cached, hit := ontologyFreshnessCache.entries[item.ID]
	ontologyFreshnessCache.Unlock()
	if hit && time.Since(cached.CheckedAt) < ontologyFreshnessTTL {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	var manifest ontologyManifest
	if err := json.Unmarshal(item.Manifest, &manifest); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stored ontology manifest is invalid"})
		return
	}

	report := ontologyFreshness{
		GeneratedAt:     manifest.GeneratedAt,
		ManifestObjects: manifest.Summary.Objects,
		CheckedAt:       time.Now().UTC(),
	}
	if !manifest.GeneratedAt.IsZero() {
		report.AgeDays = int(time.Since(manifest.GeneratedAt).Hours() / 24)
	}

	// A source that cannot be reached leaves the count unknown rather than
	// zero: "the source has 24,179 fewer objects" would be a frightening lie.
	if strings.EqualFold(strings.TrimSpace(manifest.SourceType), "dataset") {
		source, foundSource, err := h.datasetStore.GetByID(strings.TrimSpace(manifest.SourceID))
		if err == nil && foundSource && h.canReadDataset(source, identity) {
			if count, err := h.countDatasetObjects(r.Context(), source); err == nil {
				report.SourceObjects = count
				report.Drift = count - manifest.Summary.Objects
			}
		}
	}
	report.Stale = report.Drift != 0 || report.AgeDays >= 30

	ontologyFreshnessCache.Lock()
	ontologyFreshnessCache.entries[item.ID] = report
	ontologyFreshnessCache.Unlock()
	writeJSON(w, http.StatusOK, report)
}

// countDatasetObjects lists a dataset and counts. It lists; it does not read
// object contents, and it never writes: these buckets hold regulated data, and
// the platform's business with them is to say what is there.
func (h Handlers) countDatasetObjects(ctx context.Context, item dataset.Dataset) (int, error) {
	client, _, err := h.datasetS3Client(item)
	if err != nil {
		return 0, err
	}
	prefix := strings.Trim(item.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	count := 0
	for object := range client.ListObjects(listCtx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return 0, object.Err
		}
		count++
		if count > ontologyScanMaxObjects {
			break
		}
	}
	return count, nil
}
