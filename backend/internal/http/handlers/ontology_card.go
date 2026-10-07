package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// The declared card, and the control column beside it (ADR-047).
//
// The scan produces an inventory of paths and says so honestly. What it cannot
// produce is what the data *means*: what it may be used for, what it must not
// be used for, under what legal basis it was collected, or who to ask - and
// none of that is inferable from any number of bytes. So it is declared, by the
// person who knows, and this file is where that declaration is read and written.
//
// On the ontology and not on the dataset. A dataset is a bucket with
// credentials; the ontology is the layer that says what is in it, which is
// where a description belongs - and a bucket can carry several studies read
// several ways, so a card on the dataset would force one description on all of
// them. It was on the dataset first, from a misreading of ADR-043: a rescan
// replaces the manifest and keeps the ontology, so a card beside the manifest
// survives a rescan exactly as the name does.
//
// It is never returned alone. Trust does not exclude verification: every
// declared figure the platform can measure is compared with the most recent
// scan, and the verdict sits beside the declaration rather than over it.
// Replacing somebody's word with a measurement destroys the only evidence that
// they disagreed, which is the interesting fact.

type cardRequest struct {
	Study       string `json:"study"`
	Release     string `json:"release"`
	Population  string `json:"population"`
	Inclusion   string `json:"inclusion"`
	Purpose     string `json:"purpose"`
	OutOfScope  string `json:"outOfScope"`
	Limitations string `json:"limitations"`
	Provenance  string `json:"provenance"`
	LegalBasis  string `json:"legalBasis"`
	Consent     string `json:"consent"`
	Licence     string `json:"licence"`
	Units       string `json:"units"`
	Conventions string `json:"conventions"`
	Contact     string `json:"contact"`

	Subjects      *int     `json:"subjects"`
	Objects       *int64   `json:"objects"`
	Modalities    []string `json:"modalities"`
	FirstVisit    string   `json:"firstVisit"`
	LastVisit     string   `json:"lastVisit"`
	Pseudonymised *bool    `json:"pseudonymised"`
}

func cardFrom(req cardRequest) ontologydomain.Card {
	card := ontologydomain.Card{
		Study: req.Study, Release: req.Release, Population: req.Population,
		Inclusion: req.Inclusion, Purpose: req.Purpose, OutOfScope: req.OutOfScope,
		Limitations: req.Limitations, Provenance: req.Provenance,
		LegalBasis: req.LegalBasis, Consent: req.Consent, Licence: req.Licence,
		Units: req.Units, Conventions: req.Conventions, Contact: req.Contact,
		Claims: ontologydomain.Claims{
			Subjects: req.Subjects, Objects: req.Objects, Modalities: req.Modalities,
			FirstVisit: req.FirstVisit, LastVisit: req.LastVisit,
			Pseudonymised: req.Pseudonymised,
		},
	}
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

// GetOntologyCard returns what this ontology says the data means, with the
// checks beside it.
func (h Handlers) GetOntologyCard(w http.ResponseWriter, r *http.Request) {
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	measured := h.measuredFor(item)
	writeJSON(w, http.StatusOK, map[string]any{
		"declared": item.Card.Declared(),
		"card":     item.Card,
		"checks":   ontologydomain.CheckCard(item.Card, measured),
		// Named so a reader can judge the verdicts: a comparison against a
		// month-old scan is a month-old answer, however fresh the comparison.
		"measuredBy": measured.Method,
		"measuredAt": measured.At,
	})
}

// SetDatasetCard stores the declaration, or clears it with an empty body.
func (h Handlers) SetOntologyCard(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	// Who may declare is who may grant: a card is what the dataset asserts
	// about itself to everybody who can see it, which is an owner's statement
	// and not a reader's note.
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
	h.emitAudit(r, identity.UserID(), "ontology.card.declared", "ontology", item.ID, "", "success", "",
		map[string]any{"version": card.Version, "fields": declaredFields(card)})

	measured := h.measuredFor(item)
	writeJSON(w, http.StatusOK, map[string]any{
		"declared": true,
		"card":     card,
		"checks":   ontologydomain.CheckCard(&card, measured),
	})
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
	// The structure scan first, because its absence is the reason a declared
	// pseudonymisation reads as unverified rather than agreeing with a check
	// nobody ran.
	if item.Structure != nil {
		measured.IdentifyingFieldsSeen = item.Structure.CarriedIdentifiers()
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

// declaredFields names what was filled in, for the audit entry. The values are
// not audited: a card can hold a cohort's inclusion criteria, and an audit
// trail is not where that belongs.
func declaredFields(card ontologydomain.Card) []string {
	named := []struct {
		name  string
		value string
	}{
		{"study", card.Study}, {"release", card.Release}, {"population", card.Population},
		{"inclusion", card.Inclusion}, {"purpose", card.Purpose}, {"outOfScope", card.OutOfScope},
		{"limitations", card.Limitations}, {"provenance", card.Provenance},
		{"legalBasis", card.LegalBasis}, {"consent", card.Consent}, {"licence", card.Licence},
		{"units", card.Units}, {"conventions", card.Conventions}, {"contact", card.Contact},
	}
	out := []string{}
	for _, field := range named {
		if strings.TrimSpace(field.value) != "" {
			out = append(out, field.name)
		}
	}
	if card.Claims.Declared() {
		out = append(out, "claims")
	}
	return out
}
