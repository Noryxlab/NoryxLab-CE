package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
)

// What belonging gives (ADR-051).
//
// Belonging to an organization confers nothing by itself; what the
// organization gives its members is a grant. This is where an installation
// decides it once per organization rather than object by object: FOR says
// "reading" and its members read what FOR owns without an administrator
// thinking of it; Essilor leaves "nothing" and names who reads what. The
// platform writes the grant - to the organization as a subject, visible and
// revocable like any other - when an object is created for the organization
// or transferred to it. Objects already held are not touched: lowering the
// policy must not withdraw in silence.

// GetOrganizationPolicy answers the policy, "none" when nothing was set.
func (h Handlers) GetOrganizationPolicy(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	if !h.administersOrganization(identity, organizationID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "platform administrator or an administrator of this organization required"})
		return
	}
	policy := organization.Policy{OrganizationID: organizationID, MembershipGrant: organization.MembershipGrantNone}
	if h.organizationStore != nil {
		if stored, found, err := h.organizationStore.GetPolicy(organizationID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the organization policy"})
			return
		} else if found {
			policy = stored
		}
	}
	writeJSON(w, http.StatusOK, policy)
}

type organizationPolicyRequest struct {
	MembershipGrant string `json:"membershipGrant"`
}

// UpdateOrganizationPolicy sets it. Audited, because lowering it is the one
// change worth reading twice.
func (h Handlers) UpdateOrganizationPolicy(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	if !h.administersOrganization(identity, organizationID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "platform administrator or an administrator of this organization required"})
		return
	}
	if h.organizationStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "organization policies require a persistent store"})
		return
	}
	var req organizationPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !organization.ValidMembershipGrant(req.MembershipGrant) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "membershipGrant must be none, reader or writer"})
		return
	}
	previous := organization.MembershipGrantNone
	if stored, found, err := h.organizationStore.GetPolicy(organizationID); err == nil && found {
		previous = stored.MembershipGrant
	}
	item := organization.Policy{OrganizationID: organizationID, MembershipGrant: strings.ToLower(strings.TrimSpace(req.MembershipGrant)), UpdatedAt: time.Now().UTC(), UpdatedBy: identity.UserID()}
	if err := h.organizationStore.SetPolicy(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save the organization policy"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.policy.updated", "organization", organizationID, "", "success", "",
		map[string]any{"membershipGrant": item.MembershipGrant, "previous": previous})
	writeJSON(w, http.StatusOK, item)
}

// membershipGrantFor is what the policy says, "none" without a store or a row.
func (h Handlers) membershipGrantFor(organizationID string) string {
	if h.organizationStore == nil || strings.TrimSpace(organizationID) == "" {
		return organization.MembershipGrantNone
	}
	stored, found, err := h.organizationStore.GetPolicy(organizationID)
	if err != nil || !found {
		return organization.MembershipGrantNone
	}
	return stored.MembershipGrant
}

// grantWriter is how one object writes a grant: subject type, subject id,
// role. Each object has its own access store, and this is the one shape
// the settlement below needs from all of them.
type grantWriter func(subjectType, subjectID, role string) error

// settleOwnership writes the grants an ownership change implies (ADR-051).
//
// The former owner, when a person or a team, keeps the right to work:
// "owner becomes write" is a grant written in the list, visible and
// revocable, never an implicit right that would vanish. An organization that
// becomes the owner gives its members what its policy says, as a grant to
// the organization. An organization that was the owner leaves nothing
// behind: its members never held anything by owning.
//
// writeRole is the role a former owner keeps - "writer" on most objects,
// "reader" on an extract, which is frozen. A grant that fails is logged and
// does not undo the transfer: the ownership is the decision, the grant is
// its consequence, and a half-applied consequence is read on the screen.
func (h Handlers) settleOwnership(previousType, previousID, newType, newID, writeRole string, write grantWriter) {
	previousType = strings.ToLower(strings.TrimSpace(previousType))
	newType = strings.ToLower(strings.TrimSpace(newType))
	previousID, newID = strings.TrimSpace(previousID), strings.TrimSpace(newID)
	if write == nil {
		return
	}
	sameOwner := previousType == newType && strings.EqualFold(previousID, newID)
	if !sameOwner && previousID != "" && (previousType == "user" || previousType == "team") {
		_ = write(previousType, previousID, writeRole)
	}
	if newType == "organization" {
		h.applyMembershipGrant(newID, writeRole, write)
	}
}

// applyMembershipGrant writes what the organization's policy says, capped by
// what the object can take: a frozen extract gives reading at most.
func (h Handlers) applyMembershipGrant(organizationID, writeRole string, write grantWriter) {
	if write == nil {
		return
	}
	switch h.membershipGrantFor(organizationID) {
	case organization.MembershipGrantReader:
		_ = write("organization", organizationID, "reader")
	case organization.MembershipGrantWriter:
		_ = write("organization", organizationID, writeRole)
	}
}

// identityOwner is the owner a transfer is settled against when the request
// did not say: the caller themself.
func identityOwner(identity auth.Identity) (string, string) {
	return "user", identity.UserID()
}
