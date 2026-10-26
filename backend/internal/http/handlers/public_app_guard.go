package handlers

import "strings"

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

// projectHoldsRegulatedDataNamed returns the name of a regulated dataset
// attached to this project, empty when there is none.
func (h Handlers) projectHoldsRegulatedDataNamed(projectID string) string {
	if h.projectResourceStore == nil || h.datasetStore == nil {
		return ""
	}
	ids, err := h.projectResourceStore.ListProjectDatasetIDs(strings.TrimSpace(projectID))
	if err != nil {
		return ""
	}
	for _, id := range ids {
		item, found, err := h.datasetStore.GetByID(id)
		if err != nil || !found {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Classification), "hds") {
			return item.Name
		}
	}
	return ""
}
