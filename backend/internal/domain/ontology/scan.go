package ontology

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// One photograph of a source, kept after the next one replaces it.
//
// An ontology holds the latest manifest and nothing else: a rescan calls
// ReplaceManifest and the previous reading is gone (ADR-043). That is right
// for the object - an ontology is what the source looks like *now* - and
// wrong for everything somebody needs to answer afterwards. A subject count
// that moved from 2 to 31 has two possible causes, the study recruiting or
// the reading rule changing, and the platform already says which at the
// moment of the rescan; five minutes later nobody can check, because the
// thing it compared against no longer exists.
//
// It also costs an extract its explanation. An extract freezes a file list
// and records the card version it was cut against; the manifest it was cut
// from was overwritten the next time anybody pressed scan, so "why does this
// cohort have 18 patients when the ontology says 31" has no answer in the
// data.
//
// So every scan is appended, and the ontology keeps pointing at the newest.
// The manifest is an aggregate - subjects, visits, modalities, counts - not a
// file list, so a history is bounded by the shape of the study rather than by
// its size; the per-file rows stay in ontology_objects and only the current
// scan has them.
type Scan struct {
	ID         string          `json:"id"`
	OntologyID string          `json:"ontologyId"`
	Manifest   json.RawMessage `json:"manifest"`
	// Who pressed scan, and when the scan read the source. GeneratedAt comes
	// from the manifest rather than from the insert: it is the moment the
	// photograph was taken, which is the only date an extract can be reasoned
	// against.
	GeneratedBy string    `json:"generatedBy,omitempty"`
	GeneratedAt time.Time `json:"generatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ScanHistoryKept is how many photographs an ontology carries.
//
// A dataset rescanned by a cron keeps a row per run, and an unbounded table
// of manifests is a slow leak nobody watches. Fifty is far past the point
// where anybody reads a history by hand, and still cheap: these are
// aggregates, not file lists.
const ScanHistoryKept = 50

func NewScan(ontologyID string, manifest json.RawMessage, generatedBy string, generatedAt time.Time) Scan {
	now := time.Now().UTC()
	if generatedAt.IsZero() {
		generatedAt = now
	}
	return Scan{
		ID:          uuid.NewString(),
		OntologyID:  strings.TrimSpace(ontologyID),
		Manifest:    append(json.RawMessage(nil), manifest...),
		GeneratedBy: strings.TrimSpace(generatedBy),
		GeneratedAt: generatedAt.UTC(),
		CreatedAt:   now,
	}
}
