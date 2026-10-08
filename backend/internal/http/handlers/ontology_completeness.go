package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// Which subjects the study actually covers.
//
// An extract is built by asking for a modality - "every subject with a corneal
// wavefront" - and the honest answer has two halves: who has it, and who was
// expected to have it and does not. The second half is what a researcher needs
// before publishing an n, and until now the platform showed a single total
// that quietly averaged the two together.
//
// Nothing here reads the source: it is arithmetic over the stored manifest, so
// it answers for ontologies scanned months ago as readily as for this morning's.

type modalityCoverage struct {
	Name            string   `json:"name"`
	Subjects        int      `json:"subjects"`
	Objects         int      `json:"objects"`
	MissingSubjects []string `json:"missingSubjects"`
}

// formatCoverage is how much of the study each kind of file accounts for.
//
// The axis that lets a disclosure be smaller than the folder it lives in:
// on PREMYOM1000's ANTERION the measurements are 349 CSV files and 37 MB
// while the images are 20,115 files and 41 GB, so somebody who needs the
// numbers was being handed the images too - the modality was the smallest
// thing anybody could ask for.
type formatCoverage struct {
	Name       string `json:"name"`
	Objects    int    `json:"objects"`
	TotalBytes int64  `json:"totalBytes"`
}

type ontologyCompleteness struct {
	Subjects   int                `json:"subjects"`
	Modalities []modalityCoverage `json:"modalities"`
	// Formats is empty for a photograph taken before kinds were tallied. A
	// rescan fills it, and an empty list reads as "not measured" rather than
	// as "this study holds no files".
	Formats []formatCoverage `json:"formats"`
	// Subjects holding every modality the study uses. The number an extract can
	// count on without caveat.
	CompleteSubjects int `json:"completeSubjects"`
}

func (h Handlers) GetOntologyCompleteness(w http.ResponseWriter, r *http.Request) {
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
	var manifest ontologyManifest
	if err := json.Unmarshal(item.Manifest, &manifest); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stored ontology manifest is invalid"})
		return
	}
	writeJSON(w, http.StatusOK, computeOntologyCompleteness(manifest))
}

func computeOntologyCompleteness(manifest ontologyManifest) ontologyCompleteness {
	subjectsWith := map[string]map[string]bool{}
	objectsPer := map[string]int{}
	for _, subject := range manifest.Subjects {
		for _, visit := range subject.Visits {
			for _, modality := range visit.Modalities {
				name := strings.TrimSpace(modality.Name)
				if name == "" {
					continue
				}
				if subjectsWith[name] == nil {
					subjectsWith[name] = map[string]bool{}
				}
				subjectsWith[name][subject.ID] = true
				objectsPer[name] += modality.ObjectCount
			}
		}
	}

	report := ontologyCompleteness{
		Subjects:   len(manifest.Subjects),
		Modalities: []modalityCoverage{},
		Formats:    formatsIn(manifest),
	}
	for name, holders := range subjectsWith {
		coverage := modalityCoverage{
			Name:            name,
			Subjects:        len(holders),
			Objects:         objectsPer[name],
			MissingSubjects: []string{},
		}
		for _, subject := range manifest.Subjects {
			if !holders[subject.ID] {
				coverage.MissingSubjects = append(coverage.MissingSubjects, subject.ID)
			}
		}
		sort.Strings(coverage.MissingSubjects)
		report.Modalities = append(report.Modalities, coverage)
	}
	// Sparsest first: a modality two subjects out of thirty-one carry is the
	// one that decides whether an extract is worth assembling.
	sort.Slice(report.Modalities, func(i, j int) bool {
		if report.Modalities[i].Subjects != report.Modalities[j].Subjects {
			return report.Modalities[i].Subjects < report.Modalities[j].Subjects
		}
		return report.Modalities[i].Name < report.Modalities[j].Name
	})

	for _, subject := range manifest.Subjects {
		complete := true
		for _, holders := range subjectsWith {
			if !holders[subject.ID] {
				complete = false
				break
			}
		}
		if complete {
			report.CompleteSubjects++
		}
	}
	return report
}

// formatsIn totals each kind of file across the whole photograph.
func formatsIn(manifest ontologyManifest) []formatCoverage {
	tallies := map[string]ontologyFormatTally{}
	for _, subject := range manifest.Subjects {
		for _, visit := range subject.Visits {
			for _, modality := range visit.Modalities {
				for name, tally := range modality.FormatCounts {
					running := tallies[name]
					running.Objects += tally.Objects
					running.TotalBytes += tally.TotalBytes
					tallies[name] = running
				}
			}
		}
	}
	out := make([]formatCoverage, 0, len(tallies))
	for name, tally := range tallies {
		out = append(out, formatCoverage{Name: name, Objects: tally.Objects, TotalBytes: tally.TotalBytes})
	}
	// Largest first: the kind that dominates the volume is the one somebody
	// is usually trying not to take.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Objects != out[j].Objects {
			return out[i].Objects > out[j].Objects
		}
		return out[i].Name < out[j].Name
	})
	return out
}
