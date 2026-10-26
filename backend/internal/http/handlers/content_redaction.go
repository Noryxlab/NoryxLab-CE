package handlers

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// A catalogue row says an object exists. It does not carry its contents.
//
// Administering an object puts it on your screen - you have to find a thing
// to transfer it, to delete it, or to decide who reads it. What was not
// intended is that the row carry the study itself: an ontology serialises its
// manifest, which holds every subject identifier with their visits and
// modalities, and an extract serialises the subject list it froze. So a
// person named administrator of an organization, holding no grant at all,
// could read the catalogue and come away with the identifiers of every
// patient in every study that organization owns.
//
// The handlers that return contents were guarded one by one and correctly -
// `requireExtractContent` even refuses the administrator explicitly. The
// listings went around all of it, because nobody thought of a list as a
// disclosure.
//
// So contents are stripped from every row the caller may not read, at the one
// place every listing passes through. Stripping rather than hiding the row:
// the object must stay findable by whoever administers it, which was the
// whole point of showing it.

func (h Handlers) redactOntologiesForReader(items []ontologydomain.Ontology, identity auth.Identity) {
	for index := range items {
		if h.ontologyRole(items[index], identity) != "" || h.isGlobalAdmin(identity) {
			continue
		}
		items[index].Manifest = nil
		items[index].Structure = nil
	}
}

func (h Handlers) redactExtractsForReader(items []extractdomain.Extract, identity auth.Identity) {
	for index := range items {
		if h.canReadExtractByOntology(items[index], identity) {
			continue
		}
		items[index].Subjects = nil
	}
}

// canReadExtractByOntology answers the listing's question without a second
// round trip when the ontology is already out of reach.
func (h Handlers) canReadExtractByOntology(item extractdomain.Extract, identity auth.Identity) bool {
	if h.isGlobalAdmin(identity) || h.extractRole(item, identity) != "" {
		return true
	}
	if h.ontologyStore == nil {
		return false
	}
	ontology, found, err := h.ontologyStore.GetByID(item.OntologyID)
	if err != nil || !found {
		return false
	}
	return h.ontologyRole(ontology, identity) != ""
}
