package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// The declared card, and what the platform measured beside it (ADR-047).
//
// The scan produces an inventory of paths and says so honestly. What it cannot
// produce is what the data *means*: what it may be used for, what it must not
// be used for, under what right it is held, who to ask - and none of that is
// inferable from any number of bytes. So it is declared, by the person who
// knows, in one paragraph.
//
// On the ontology and not on the dataset. A dataset is a bucket with
// credentials; the ontology is the layer that says what is in it, which is
// where a description belongs - and a bucket can carry several studies read
// several ways, so a card on the dataset would force one description on all of
// them. It was on the dataset first, from a misreading of ADR-043: a rescan
// replaces the manifest and keeps the ontology, so a card beside the manifest
// survives a rescan exactly as the name does.
//
// It is never returned alone. Trust does not exclude verification, so the
// figures the platform took itself travel with the declaration - with no
// verdicts, because prose cannot be checked. Here is what somebody wrote, here
// is what we counted and when, and here is whether anything has ever looked
// inside the files.

type cardRequest struct {
	Text string `json:"text"`
}

func cardFrom(req cardRequest) ontologydomain.Card {
	card := ontologydomain.Card{Text: req.Text}
	card.Normalise()
	return card
}

// requireOntologyForCard resolves the ontology and refuses it to somebody who
// cannot see it - 404 rather than 403, following the rest of this package:
// whether an ontology exists is itself information over regulated data.
func (h Handlers) requireOntologyForCard(w http.ResponseWriter, r *http.Request) (ontologydomain.Ontology, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return ontologydomain.Ontology{}, false
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return ontologydomain.Ontology{}, false
	}
	if !h.isGlobalAdmin(identity) && h.ontologyRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return ontologydomain.Ontology{}, false
	}
	return item, true
}

// GetOntologyCard returns what this ontology says the data means, with what
// the platform measured beside it.
func (h Handlers) GetOntologyCard(w http.ResponseWriter, r *http.Request) {
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, cardPayload(item, h.measuredFor(item)))
}

// SetOntologyCard stores the declaration, or clears it with an empty body.
func (h Handlers) SetOntologyCard(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	// Who may declare is who may grant: a card is what this ontology asserts
	// to everybody who can see it, which is an owner's statement and not a
	// reader's note.
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
		return
	}

	var req cardRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid card"})
			return
		}
	}
	card := cardFrom(req)
	if !card.Declared() {
		if err := h.ontologyStore.SetCard(item.ID, nil); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear the card"})
			return
		}
		h.emitAudit(r, identity.UserID(), "ontology.card.cleared", "ontology", item.ID, "", "success", "", nil)
		writeJSON(w, http.StatusOK, map[string]any{"declared": false})
		return
	}

	// The version is the previous one plus one, and the writer is recorded.
	// A declaration is somebody's word, so it carries whose - and an extract
	// cut against version 3 can say so even after version 4 is written.
	card.Version = 1
	if item.Card != nil {
		card.Version = item.Card.Version + 1
	}
	card.DeclaredBy = identity.UserID()
	card.DeclaredAt = time.Now().UTC()

	if err := h.ontologyStore.SetCard(item.ID, &card); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the card"})
		return
	}
	// The length and not the text: a card can hold a cohort's inclusion
	// criteria, and an audit trail is not where that belongs.
	h.emitAudit(r, identity.UserID(), "ontology.card.declared", "ontology", item.ID, "", "success", "",
		map[string]any{"version": card.Version, "characters": len(card.Text)})

	item.Card = &card
	writeJSON(w, http.StatusOK, cardPayload(item, h.measuredFor(item)))
}

// measuredFor is what this ontology's own passes measured.
//
// Its own manifest, which is simpler and truer than the previous version: that
// one searched for "the most recent ontology over this dataset", which answered
// a question nobody asked when a bucket carried two studies read two ways. An
// ontology is a photograph; the card beside it is checked against that
// photograph and no other.
func (h Handlers) measuredFor(item ontologydomain.Ontology) ontologydomain.Measured {
	measured := ontologydomain.Measured{}
	// The structure scan first, because whether anything has ever looked inside
	// the files is the question a count of objects cannot answer.
	if item.Structure != nil {
		measured.IdentifyingFieldsSeen = item.Structure.CarriedIdentifiers()
		measured.IdentifyingChecked = item.Structure.IdentifyingChecked
		measured.IdentifyingPresent = item.Structure.IdentifyingPresent
		measured.IdentifyingMethod = item.Structure.Method
		measured.IdentifyingAt = item.Structure.At
	}
	var manifest ontologyManifest
	if err := json.Unmarshal(item.Manifest, &manifest); err != nil {
		return measured
	}
	measured.Subjects = manifest.Summary.Subjects
	measured.Objects = int64(manifest.Summary.Objects)
	measured.Method = "path scan"
	measured.At = manifest.GeneratedAt
	measured.Modalities = modalitiesIn(manifest)
	measured.FirstVisit, measured.LastVisit = visitRange(manifest)
	return measured
}

func modalitiesIn(manifest ontologyManifest) []string {
	seen := map[string]bool{}
	for _, subject := range manifest.Subjects {
		for _, visit := range subject.Visits {
			for _, modality := range visit.Modalities {
				name := strings.TrimSpace(modality.Name)
				if name != "" {
					seen[name] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// visitRange is the earliest and latest visit date the keys carry. Lexical
// comparison, which is correct because the dates are AAAAMMJJ - and anything
// that is not eight digits is not a date this can order, so it is left out
// rather than guessed at.
func visitRange(manifest ontologyManifest) (string, string) {
	first, last := "", ""
	for _, subject := range manifest.Subjects {
		for _, visit := range subject.Visits {
			date := strings.TrimSpace(visit.Date)
			if !ontologyDatePattern.MatchString(date) {
				continue
			}
			if first == "" || date < first {
				first = date
			}
			if last == "" || date > last {
				last = date
			}
		}
	}
	return first, last
}

// cardPayload is what both endpoints answer: the declaration, and what the
// platform measured beside it.
func cardPayload(item ontologydomain.Ontology, measured ontologydomain.Measured) map[string]any {
	return map[string]any{
		"declared": item.Card.Declared(),
		"card":     item.Card,
		"measured": map[string]any{
			"subjects":   measured.Subjects,
			"objects":    measured.Objects,
			"modalities": measured.Modalities,
			"firstVisit": measured.FirstVisit,
			"lastVisit":  measured.LastVisit,
			"method":     measured.Method,
			"at":         measured.At,
		},
		"structure": map[string]any{
			"ran":                measured.IdentifyingFieldsSeen != nil,
			"identifiersPresent": measured.IdentifyingFieldsSeen,
			"checked":            measured.IdentifyingChecked,
			"present":            measured.IdentifyingPresent,
			"method":             measured.IdentifyingMethod,
			"at":                 measured.IdentifyingAt,
		},
	}
}
