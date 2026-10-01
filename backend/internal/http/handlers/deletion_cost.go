package handlers

import (
	"net/http"
	"strings"
)

// What deleting something takes with it.
//
// On 2026-10-01 somebody deleted an ontology to rescan it - the habit from
// before a rescan refreshed in place - and a 4,019-file extract went with it.
// The confirmation said "delete the ontology" and nothing else. An object whose
// removal destroys three others in silence is a loss discovered afterwards,
// and the only thing standing between a person and it was knowing the schema.
//
// Two cascades, and they differ in a way worth saying out loud:
//
//   - deleting an **ontology** destroys its extracts, because an extract is a
//     selection over that ontology's subjects and means nothing without it;
//   - deleting a **dataset** destroys nothing, and leaves its ontologies
//     describing a source that no longer exists - which is worse in its own
//     way, because the screen still shows them and they still look usable.
//
// Asked when somebody clicks delete rather than computed for every row: the
// answer costs a query per object, and a list of forty would pay for forty
// answers nobody reads.

type deletionCost struct {
	Ontologies int `json:"ontologies,omitempty"`
	Extracts   int `json:"extracts,omitempty"`
	Objects    int `json:"objects,omitempty"`
}

// GetOntologyDeletionCost says what deleting this ontology destroys.
func (h Handlers) GetOntologyDeletionCost(w http.ResponseWriter, r *http.Request) {
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
	cost := deletionCost{}
	if h.extractStore != nil {
		if extracts, err := h.extractStore.ListByOntology(item.ID); err == nil {
			cost.Extracts = len(extracts)
		}
	}
	if count, err := h.ontologyStore.CountObjects(item.ID); err == nil {
		cost.Objects = count
	}
	writeJSON(w, http.StatusOK, cost)
}

// GetDatasetDeletionCost says what deleting this dataset leaves behind.
//
// Nothing cascades: the ontologies survive, describing a bucket that is gone.
// They keep appearing in the catalogue and keep looking usable, and the first
// sign is a rescan that cannot find its source.
func (h Handlers) GetDatasetDeletionCost(w http.ResponseWriter, r *http.Request) {
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
	cost := deletionCost{}
	ontologies, err := h.ontologiesVisibleTo(identity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read ontologies"})
		return
	}
	for _, ontology := range ontologies {
		if strings.TrimSpace(ontology.SourceID) != item.ID {
			continue
		}
		cost.Ontologies++
		if h.extractStore == nil {
			continue
		}
		if extracts, err := h.extractStore.ListByOntology(ontology.ID); err == nil {
			cost.Extracts += len(extracts)
		}
	}
	writeJSON(w, http.StatusOK, cost)
}
