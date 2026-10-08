package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// The scans an ontology has been through.
//
// An ontology holds the latest manifest and nothing else: a rescan replaces
// the picture and keeps the object (ADR-043). That is right for the object -
// an ontology is what the source looks like now - and leaves two questions
// unanswerable five minutes later.
//
// A subject count that moved from 2 to 31 has two causes, the study
// recruiting or the reading rule changing, and they call for opposite
// reactions. The platform says which at the moment of the rescan, then throws
// away what it compared against. And an extract freezes a file list against a
// manifest that the next scan overwrites, so "why does this cohort have 18
// patients when the ontology says 31" has no answer in the data.
//
// What is returned is the summary of each photograph, not the photograph:
// a client listing a history wants dates, counts and the rule that produced
// them. The manifests themselves stay in the database, which is what makes a
// future "read this ontology as it stood in March" possible at all.

type ontologyScanEntry struct {
	ID          string `json:"id"`
	GeneratedAt string `json:"generatedAt"`
	GeneratedBy string `json:"generatedBy,omitempty"`
	Study       string `json:"study,omitempty"`
	Subjects    int    `json:"subjects"`
	Visits      int    `json:"visits"`
	Modalities  int    `json:"modalities"`
	Objects     int    `json:"objects"`
	Unreadable  int    `json:"unreadable"`
	TotalBytes  int64  `json:"totalBytes"`
	// Reading is the rule that produced this photograph, in positions and
	// names rather than prose: a history whose rows cannot be told apart by
	// their reading cannot answer the only question it exists for.
	Reading ontologyReadingRule `json:"reading"`
	// Changes against the scan before it, which is what a reader is actually
	// comparing. Absent on the oldest row, because there is nothing to
	// compare it against - and inventing a baseline of zero would report a
	// first scan as a study that gained thirty-one subjects.
	Difference *ontologyRefreshDiff `json:"difference,omitempty"`
}

// GetOntologyScans lists the photographs, newest first.
func (h Handlers) GetOntologyScans(w http.ResponseWriter, r *http.Request) {
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	scans, err := h.ontologyStore.ListOntologyScans(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the scan history"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": ontologyScanEntries(scans)})
}

// ontologyScanEntries summarises each scan and compares it with the one
// before it in time.
func ontologyScanEntries(scans []ontologydomain.Scan) []ontologyScanEntry {
	// Newest first is how it is read; oldest first is how a difference is
	// computed. Sorted here rather than trusted from the store, so the
	// comparison cannot silently invert.
	ordered := append([]ontologydomain.Scan(nil), scans...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].GeneratedAt.Before(ordered[j].GeneratedAt)
	})

	entries := make([]ontologyScanEntry, 0, len(ordered))
	for index, scan := range ordered {
		var manifest ontologyManifest
		if err := json.Unmarshal(scan.Manifest, &manifest); err != nil {
			// A manifest that will not decode is still a scan that happened,
			// and dropping the row would make the history lie about how many
			// times the source was read.
			entries = append(entries, ontologyScanEntry{
				ID:          scan.ID,
				GeneratedAt: scan.GeneratedAt.Format(time.RFC3339),
				GeneratedBy: scan.GeneratedBy,
			})
			continue
		}
		entry := ontologyScanEntry{
			ID:          scan.ID,
			GeneratedAt: scan.GeneratedAt.Format(time.RFC3339),
			GeneratedBy: strings.TrimSpace(scan.GeneratedBy),
			Study:       strings.TrimSpace(manifest.Study),
			Subjects:    manifest.Summary.Subjects,
			Visits:      manifest.Summary.Visits,
			Modalities:  manifest.Summary.Modalities,
			Objects:     manifest.Summary.Objects - manifest.Summary.Unrecognised,
			Unreadable:  manifest.Summary.Unrecognised,
			TotalBytes:  manifest.Summary.TotalBytes,
			Reading:     manifest.ReadingRule,
		}
		if index > 0 {
			entry.Difference = compareOntologyScans(ordered[index-1].Manifest, manifest)
		}
		entries = append(entries, entry)
	}

	// Back to newest first for the caller.
	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}
	return entries
}
