package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Recording what one audited pass over the files found (ADR-047).
//
// On the ontology, beside the manifest, because both are measurements of the
// same subject: one reads keys, the other reads inside files.
//
// The path scan reads keys and this reads inside files, which is a different
// act over regulated data. So it is not a scan the platform launches on its
// own: a scanner runs it, under an allowlist, and reports here. That split is
// deliberate - whoever opens the files is a workload somebody started, and this
// endpoint is the one line in the audit trail that says it happened.
//
// What arrives is checked rather than trusted. A scanner is a client, and the
// two rules that make this safe are enforced on this side: a field read to be
// checked is never a field read to be kept, and a value set too wide to be a
// distribution loses its values.

type structureScanRequest struct {
	Allowlists         []ontologydomain.StructureAllowlist `json:"allowlists"`
	Objects            int                                 `json:"objects"`
	Formats            map[string]int                      `json:"formats"`
	Tags               map[string][]string                 `json:"tags"`
	Tallies            []ontologydomain.FieldTally         `json:"tallies"`
	IdentifyingChecked []string                            `json:"identifyingChecked"`
	IdentifyingPresent []string                            `json:"identifyingPresent"`
	Unreadable         int                                 `json:"unreadable"`
	Method             string                              `json:"method"`
}

// SetDatasetStructureScan records a pass, or clears the record with an empty
// body.
func (h Handlers) SetOntologyStructureScan(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	// Who may record is who may grant. A structure scan asserts something
	// about the data to everybody who can see it - and on a regulated
	// bucket, "no identifier was found" is the most consequential sentence the
	// platform can hold.
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
		return
	}

	var req structureScanRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid structure scan"})
			return
		}
	}
	if len(req.Allowlists) == 0 {
		if err := h.ontologyStore.SetStructureScan(item.ID, nil); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear the structure scan"})
			return
		}
		h.emitAudit(r, identity.UserID(), "ontology.structure_scan.cleared", "ontology", item.ID, "", "success", "", nil)
		writeJSON(w, http.StatusOK, map[string]any{"recorded": false})
		return
	}

	// The allowlist is checked before anything it produced is stored. A field
	// in both lists would have been read to be checked and read to be kept,
	// and on a regulated bucket that is how an identifier reaches a card.
	for _, allowlist := range req.Allowlists {
		if problem := allowlist.Problem(); problem != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": problem})
			return
		}
	}
	// A recorded field must not be one of the identifying fields of any
	// allowlist, not only of its own: a scanner that split them across two
	// formats would otherwise slip past the check above.
	identifying := map[string]bool{}
	for _, allowlist := range req.Allowlists {
		for _, field := range allowlist.Identifying {
			identifying[strings.ToLower(strings.TrimSpace(field))] = true
		}
	}
	for _, tally := range req.Tallies {
		if identifying[strings.ToLower(strings.TrimSpace(tally.Field))] {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "field " + tally.Field + " is identifying somewhere in this allowlist; its values are not recorded",
			})
			return
		}
	}

	scan := ontologydomain.StructureScan{
		Allowlists: req.Allowlists, Objects: req.Objects, Formats: req.Formats,
		Tags: req.Tags, Tallies: req.Tallies,
		IdentifyingChecked: req.IdentifyingChecked,
		IdentifyingPresent: req.IdentifyingPresent,
		Unreadable:         req.Unreadable,
		RanBy:              identity.UserID(),
		At:                 time.Now().UTC(),
		Method:             strings.TrimSpace(req.Method),
	}
	if scan.Method == "" {
		scan.Method = "structure scan"
	}
	// Normalise applies the cap on this side too, because the scanner is a
	// client: whatever it sends, a field with too many values does not keep
	// them.
	scan.Normalise()

	if err := h.ontologyStore.SetStructureScan(item.ID, &scan); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the structure scan"})
		return
	}
	// Audited with the figures that matter and none of the values: how many
	// files were opened, which identifying fields were looked at, and which of
	// them held something. That last list is the point of the whole pass.
	h.emitAudit(r, identity.UserID(), "ontology.structure_scan.recorded", "ontology", item.ID, "", "success", "",
		map[string]any{
			"objects":            scan.Objects,
			"unreadable":         scan.Unreadable,
			"identifyingChecked": scan.IdentifyingChecked,
			"identifyingPresent": scan.IdentifyingPresent,
			"method":             scan.Method,
		})

	writeJSON(w, http.StatusOK, map[string]any{"recorded": true, "structure": scan})
}

// GetDatasetStructureScan returns the last pass, if there was one.
func (h Handlers) GetOntologyStructureScan(w http.ResponseWriter, r *http.Request) {
	item, ok := h.requireOntologyForCard(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"recorded":  item.Structure != nil,
		"structure": item.Structure,
	})
}
