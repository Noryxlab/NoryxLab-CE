package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// Who a catalogue object can belong to, in one place.
//
// Four objects answered this question in four copies, and the copies had
// already drifted: a dataset knew about teams as access subjects while an
// ontology did not, so an ontology could not even be shared with a team. The
// drift is the argument for one function - "the same everywhere" is a property
// of the code before it is a property of the product.
//
// Three kinds, and the difference between them is who answers for the data. A
// person answers for themselves. An organization answers as a legal entity. A
// team is a working group inside one, and owning is how a group of people who
// work together keep what they produce when one of them leaves.
const (
	ownerUser         = "user"
	ownerTeam         = "team"
	ownerOrganization = "organization"
)

// normaliseOwner validates an owner, resolves a name to an identifier, and
// checks that the caller may hand the object there.
//
// Returns the normalised pair, then an HTTP status and a message when the
// request has to be refused. Status zero means accepted.
//
// Giving something away to a group you do not belong to is the case this
// guards: it dispossesses the caller without anybody having asked to receive,
// and the object becomes unreachable to everyone who could still manage it.
// A global administrator is exempt, because somebody has to be able to repair
// an orphan.
func (h Handlers) normaliseOwner(ownerType, ownerID string, identity auth.Identity) (string, string, int, string) {
	kind := strings.ToLower(strings.TrimSpace(ownerType))
	id := strings.TrimSpace(ownerID)
	refus := "ownerType must be user, team or organization and ownerId is required"
	if id == "" {
		return "", "", http.StatusBadRequest, refus
	}

	switch kind {
	case ownerUser:
		return kind, id, 0, ""

	case ownerOrganization:
		organization, found := h.resolveOrganization(id)
		if !found {
			return "", "", http.StatusBadRequest, "no organization named " + id
		}
		id = organization.ID
		if h.isGlobalAdmin(identity) || h.callerInOrganization(identity, id) {
			return kind, id, 0, ""
		}
		return "", "", http.StatusForbidden,
			"destination organization membership or global admin required"

	case ownerTeam:
		item, found := h.resolveTeam(id)
		if !found {
			return "", "", http.StatusBadRequest, "no team named " + id
		}
		id = item.ID
		if h.isGlobalAdmin(identity) {
			return kind, id, 0, ""
		}
		for _, teamID := range h.callerTeamIDs(identity) {
			if teamID == id {
				return kind, id, 0, ""
			}
		}
		return "", "", http.StatusForbidden,
			"destination team membership or global admin required"
	}
	return "", "", http.StatusBadRequest, refus
}

// callerInOrganization reports membership of the destination organization.
func (h Handlers) callerInOrganization(identity auth.Identity, organizationID string) bool {
	for _, subject := range h.ontologySubjects(identity) {
		if subject.Type == ownerOrganization && subject.ID == organizationID {
			return true
		}
	}
	return false
}

// allOrganizationIDs lists what the directory knows, for resolving a team name.
func (h Handlers) allOrganizationIDs() []string {
	if h.keycloak == nil {
		return nil
	}
	organizations, err := h.keycloak.ListOrganizations()
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(organizations))
	for _, organization := range organizations {
		ids = append(ids, organization.ID)
	}
	return ids
}
