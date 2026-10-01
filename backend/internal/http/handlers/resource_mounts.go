package handlers

import (
	"net/http"
	"sort"
	"strings"
)

// Which projects mount a catalogue object.
//
// Attaching only ever had one direction on screen. A project listed what it
// mounted; an object said nothing about where it was mounted, and offered no
// way to put it anywhere. Somebody looking at an extract and wanting it in
// their project had to know to go to the project instead - and for an extract
// that is exactly backwards, because it exists to be reused by several
// projects. That is the whole point of a link rather than an appartenance
// (ADR-042).
//
// The question is answered for the three objects that attach the same way:
// a dataset, an ontology and an extract. A datasource already had its own
// reverse read for the connection screen.

type mountedProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// projectsMounting names the projects the caller may know about, and no
// others.
//
// A project name is not public: it carries a customer, a study, sometimes a
// person. So a caller is told about the projects they are a member of, and an
// administrator about all of them. The count of the rest is returned too -
// "mounted in two projects you cannot see" is a true and useful answer, where
// silence would read as "mounted nowhere" and invite somebody to delete it.
func (h Handlers) projectsMounting(projectIDs []string, userID string, isAdmin bool) ([]mountedProject, int) {
	visible := []mountedProject{}
	hidden := 0
	for _, projectID := range projectIDs {
		projectID = strings.TrimSpace(projectID)
		if projectID == "" {
			continue
		}
		if !isAdmin && !h.hasProjectMembership(userID, projectID) {
			hidden++
			continue
		}
		item, found, err := h.projectByID(projectID)
		if err != nil || !found {
			// A link to a project that no longer exists. Counted rather than
			// named: it is the orphan the platform validator reports, and it
			// is not something a person can act on from this screen.
			hidden++
			continue
		}
		visible = append(visible, mountedProject{ID: item.ID, Name: item.Name})
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].Name < visible[j].Name })
	return visible, hidden
}

func (h Handlers) writeMounts(w http.ResponseWriter, projectIDs []string, userID string, isAdmin bool) {
	items, hidden := h.projectsMounting(projectIDs, userID, isAdmin)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "hidden": hidden})
}

// ListDatasetProjects answers "where is this dataset mounted".
func (h Handlers) ListDatasetProjects(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.isGlobalAdmin(identity) && h.datasetRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	projectIDs, err := h.projectResourceStore.ListDatasetProjectIDs(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read project links"})
		return
	}
	h.writeMounts(w, projectIDs, identity.UserID(), h.isGlobalAdmin(identity))
}

// ListOntologyProjects answers "where is this ontology mounted".
func (h Handlers) ListOntologyProjects(w http.ResponseWriter, r *http.Request) {
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
	projectIDs, err := h.projectResourceStore.ListOntologyProjectIDs(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read project links"})
		return
	}
	h.writeMounts(w, projectIDs, identity.UserID(), h.isGlobalAdmin(identity))
}

// ListExtractProjects answers "where is this extract mounted".
//
// Visibility is the ontology's, as everywhere else an extract is listed: its
// name and its n describe that ontology's content.
func (h Handlers) ListExtractProjects(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	projectIDs, err := h.projectResourceStore.ListExtractProjectIDs(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read project links"})
		return
	}
	h.writeMounts(w, projectIDs, identity.UserID(), h.isGlobalAdmin(identity))
}
