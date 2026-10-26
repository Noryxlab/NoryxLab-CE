package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	organizationdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
)

// Who administers what an organization owns.
//
// The platform used to answer this from the directory: belong to Essilor, and
// you administered everything Essilor owned - nine datasets out of twelve
// here, eight of them regulated. That read an identity fact as a permission,
// and the two are not the same thing. Belonging says where somebody works.
// Administering is a right, and a right is granted by somebody who can be
// named.
//
// So an organization names its administrators here, and nothing else confers
// it. Until it names somebody, only a platform administrator acts for it -
// the direction that fails closed.

type organizationAdminRequest struct {
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
}

// ListOrganizationAdmins answers who may act for this organization.
func (h Handlers) ListOrganizationAdmins(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "users"); !ok {
		return
	}
	if h.organizationStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []organizationdomain.Admin{}})
		return
	}
	organizationID, ok := h.resolveOrganizationID(w, r)
	if !ok {
		return
	}
	items, err := h.organizationStore.ListAdmins(organizationID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list the organization's administrators"})
		return
	}
	h.nameOrganizationAdmins(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AddOrganizationAdmin names a person or a team as administrator.
func (h Handlers) AddOrganizationAdmin(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.organizationStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "organization administrators require a persistent store"})
		return
	}
	organizationID, ok := h.resolveOrganizationID(w, r)
	if !ok {
		return
	}
	subjectType := strings.ToLower(strings.TrimSpace(r.PathValue("subjectType")))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	if subjectType == "" || subjectID == "" {
		var req organizationAdminRequest
		if r.Body != nil && r.Body != http.NoBody {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		subjectType = strings.ToLower(strings.TrimSpace(req.SubjectType))
		subjectID = strings.TrimSpace(req.SubjectID)
	}
	if !organizationdomain.AdministrableSubjectType(subjectType) || subjectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subjectType must be user or team, with a subjectId"})
		return
	}
	// A team is resolved to its identifier, like every other grant: the screen
	// shows names, and a name stored as an identifier is a grant that stops
	// applying the day somebody renames the team.
	if subjectType == "team" {
		team, found := h.resolveTeam(subjectID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no team with identifier " + subjectID})
			return
		}
		subjectID = team.ID
	}
	item := organizationdomain.Admin{
		OrganizationID: organizationID,
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := h.organizationStore.AddAdmin(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to name the administrator"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.admin.grant", "organization", organizationID, "", "success", "",
		map[string]any{"subjectType": subjectType, "subjectId": subjectID})
	writeJSON(w, http.StatusOK, item)
}

// RemoveOrganizationAdmin takes that right back.
func (h Handlers) RemoveOrganizationAdmin(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.organizationStore == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	organizationID, ok := h.resolveOrganizationID(w, r)
	if !ok {
		return
	}
	subjectType := strings.ToLower(strings.TrimSpace(r.PathValue("subjectType")))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	if subjectType == "team" {
		if team, found := h.resolveTeam(subjectID); found {
			subjectID = team.ID
		}
	}
	if err := h.organizationStore.RemoveAdmin(organizationID, subjectType, subjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove the administrator"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.admin.revoke", "organization", organizationID, "", "success", "",
		map[string]any{"subjectType": subjectType, "subjectId": subjectID})
	w.WriteHeader(http.StatusNoContent)
}

// resolveOrganizationID accepts the alias a screen shows as readily as the
// identifier it stores.
func (h Handlers) resolveOrganizationID(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.PathValue("organizationID"))
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organizationID is required"})
		return "", false
	}
	if organization, found := h.resolveOrganization(raw); found {
		return organization.ID, true
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "no organization named " + raw})
	return "", false
}
