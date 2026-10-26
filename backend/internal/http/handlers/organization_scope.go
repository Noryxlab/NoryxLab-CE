package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// Administration, within a perimeter.
//
// Every administration screen asked one question - "platform administrator?" -
// and answered the whole platform or nothing. So one person carried every
// account, every team and every transfer for every organization on the
// installation, and the customers who wanted to run their own side had no way
// to be given it short of the keys to everything.
//
// An organization names its administrators (ADR-048); these are the checks
// that let those people act, and only inside what they were named for. The
// platform administrator passes every one of them, which is what keeps the
// installation from ever being stuck.

// administersOrganization reports whether this caller may act for one
// organization.
func (h Handlers) administersOrganization(identity auth.Identity, organizationID string) bool {
	if h.isGlobalAdmin(identity) {
		return true
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return false
	}
	for _, candidate := range h.administeredOrganizationIDs(identity) {
		if strings.EqualFold(candidate, organizationID) {
			return true
		}
	}
	return false
}

// requireOrganizationScope admits a platform administrator, or somebody this
// organization named, and refuses everybody else in the words the rest of the
// administration API uses.
func (h Handlers) requireOrganizationScope(w http.ResponseWriter, r *http.Request, organizationID string) (auth.Identity, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return auth.Identity{}, false
	}
	if h.administersOrganization(identity, organizationID) {
		return identity, true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error": "administering this organization requires being named one of its administrators, or the platform admin role",
	})
	return auth.Identity{}, false
}

// requireTeamScope is the same check for the routes that name a team rather
// than its organization.
//
// The team is resolved first and its organization read from the record, never
// from the request: a caller who could name the organization a team belongs to
// could name one they administer and reach somebody else's team.
func (h Handlers) requireTeamScope(w http.ResponseWriter, r *http.Request, teamID string) (auth.Identity, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return auth.Identity{}, false
	}
	if h.isGlobalAdmin(identity) {
		return identity, true
	}
	if h.teamStore == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required"})
		return auth.Identity{}, false
	}
	item, found, err := h.teamStore.GetByID(strings.TrimSpace(teamID))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "team not found"})
		return auth.Identity{}, false
	}
	if h.administersOrganization(identity, item.OrganizationID) {
		return identity, true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error": "administering this team requires being named an administrator of its organization, or the platform admin role",
	})
	return auth.Identity{}, false
}

// organizationsInScope is which organizations this caller's screens may show.
//
// Nil means "every one of them", for a platform administrator. Anything else
// is the list they were named for, and a screen that receives it must filter
// rather than assume: a page that forgets its perimeter shows one customer to
// another, which is worse than the problem this solves.
func (h Handlers) organizationsInScope(identity auth.Identity) []string {
	if h.isGlobalAdmin(identity) {
		return nil
	}
	return h.administeredOrganizationIDs(identity)
}
