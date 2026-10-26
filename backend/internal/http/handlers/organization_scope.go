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

// teamAdditionIsAnEscalation says why adding somebody to this team would hand
// out more than team membership, empty when it would not.
//
// Two paths turn "manage your organization's teams" into something larger. A
// team named an administrator of an organization: adding a member to it names
// an administrator, which ADR-048 keeps with the platform administrator. And a
// team that owns or was granted regulated data: adding a member opens that
// data, while granting a read on a regulated dataset directly is reserved to
// the platform administrator. Both would convert administering into reading
// by a side door.
func (h Handlers) teamAdditionIsAnEscalation(teamID string) string {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return ""
	}
	if h.organizationStore != nil {
		if ids, err := h.organizationStore.AdministeredBy(nil, []string{teamID}); err == nil && len(ids) > 0 {
			return "this team administers an organization; naming an administrator is a platform administrator's act"
		}
	}
	if h.datasetStore == nil {
		return ""
	}
	items, err := h.datasetStore.ListAll()
	if err != nil {
		// Refuser plutot que deviner : une liste illisible ne doit pas
		// autoriser l'ajout qu'elle servait a juger.
		return "the datasets this team holds cannot be read right now; try again"
	}
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Classification), "hds") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.OwnerType), "team") &&
			strings.EqualFold(strings.TrimSpace(item.OwnerID), teamID) {
			return "this team owns regulated data (" + item.Name + "); adding a member would open it, which is a platform administrator's act"
		}
		if access, found, err := h.datasetStore.GetAccess(item.ID, "team", teamID); err == nil && found && strings.TrimSpace(access.Role) != "" {
			return "this team holds a grant on regulated data (" + item.Name + "); adding a member would open it, which is a platform administrator's act"
		}
	}
	return ""
}
