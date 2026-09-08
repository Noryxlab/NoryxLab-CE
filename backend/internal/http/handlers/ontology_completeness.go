package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// Which subjects the study actually covers.
//
// A cohort is built by asking for a modality - "every subject with a corneal
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

type ontologyCompleteness struct {
	Subjects   int                `json:"subjects"`
	Modalities []modalityCoverage `json:"modalities"`
	// Subjects holding every modality the study uses. The number a cohort can
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

	report := ontologyCompleteness{Subjects: len(manifest.Subjects), Modalities: []modalityCoverage{}}
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
	// one that decides whether a cohort is worth assembling.
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
