package handlers

import (
	"strings"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Regulated data and a publicly served application cannot share a project.
//
// An application serves whatever its code chooses to render, to whoever its
// access mode admits - and "public" admits the internet, with no session at
// all. The pod itself mounts the project's datasets, resolved for the person
// who launched it, so an application published by somebody who may read a
// regulated bucket has that bucket mounted behind a URL that asks nobody for
// anything.
//
// Nothing refused it. A researcher could expose a health study with one form,
// deliberately or by pasting the wrong default, and the platform would not
// even record who looked: an anonymous view carries no actor.
//
// So the two are kept apart, and whichever arrives second is refused by name.
// Publishing to the public is a real need - a dashboard for a funder, a
// figure for a paper - and so is mounting regulated data; what is not a need
// is doing both in one project.

const publicAppAccessMode = "public"

// projectServesPubliclyNamed returns the name of a publicly served application
// in this project, empty when there is none.
func (h Handlers) projectServesPubliclyNamed(projectID string) string {
	if h.appStore == nil {
		return ""
	}
	items, err := h.appStore.List()
	if err != nil {
		return ""
	}
	for _, item := range items {
		if strings.TrimSpace(item.ProjectID) != strings.TrimSpace(projectID) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.AccessMode), publicAppAccessMode) {
			return item.Name
		}
	}
	return ""
}

// projectHoldsRegulatedDataNamed returns the name of regulated data reachable
// from this project, empty when there is none.
//
// Two doors, not one. A dataset is attached directly; an extract is attached
// on its own and the filler fetches its files from the source bucket without
// that dataset ever being attached - which is the whole point of an extract,
// and which would have walked straight around a guard that only looked at
// datasets.
func (h Handlers) projectHoldsRegulatedDataNamed(projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if h.projectResourceStore == nil {
		return ""
	}
	if h.datasetStore != nil {
		if ids, err := h.projectResourceStore.ListProjectDatasetIDs(projectID); err == nil {
			for _, id := range ids {
				item, found, err := h.datasetStore.GetByID(id)
				if err != nil || !found {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(item.Classification), "hds") {
					return item.Name
				}
			}
		}
	}
	if h.extractStore == nil || h.ontologyStore == nil {
		return ""
	}
	for _, extract := range h.projectExtractsQuiet(projectID) {
		ontology, found, err := h.ontologyStore.GetByID(extract.OntologyID)
		if err != nil || !found {
			continue
		}
		if h.ontologyReadsRegulatedData(ontology) {
			return extract.Name
		}
	}
	return ""
}

// projectExtractsQuiet lists what the project attached, without the error
// path: a guard that fails open on a listing error would be no guard.
func (h Handlers) projectExtractsQuiet(projectID string) []extractdomain.Extract {
	items, err := h.projectExtracts(projectID)
	if err != nil {
		return nil
	}
	return items
}

// ontologyReadsRegulatedData reports whether this ontology was built over a
// regulated dataset.
func (h Handlers) ontologyReadsRegulatedData(ontology ontologydomain.Ontology) bool {
	if h.datasetStore == nil || !strings.EqualFold(strings.TrimSpace(ontology.SourceType), "dataset") {
		return false
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(ontology.SourceID))
	if err != nil || !found {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(item.Classification), "hds")
}
