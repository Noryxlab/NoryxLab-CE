package handlers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

// Who holds a role on a project, and the ability to take it back.
//
// There was no way to ask. A role could be given - `PUT .../members/{userID}/role`
// - and never listed, so the screen built its table from the one name it had,
// the project's owner. Everybody else granted access was invisible.
//
// On 2026-11-13 Cédric Folignoni reported a test account reaching a project he
// could not explain: the members screen showed one person, the owner, while
// `access_roles` held three. His access was real and explicit - somebody had
// granted it - and the platform had no screen that could say so. Worse, it had
// no way to revoke it either: the only writing route sets a role, and setting
// an empty one left the row in place.
//
// On a platform sold for auditability, a grant that can be created and neither
// read nor withdrawn is the defect, not the grant.
//
// What this answers is the direct, personal grants and the owner. Grants that
// reach somebody through a team or an organization have their own screens and
// their own endpoints, and flattening them here would hide where they come
// from - which is the mistake being corrected, not one to repeat.

type projectMember struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	// Owner marks the grant nobody gave and nobody can take away: ownership is
	// changed through its own route, so a screen must not offer to revoke it.
	Owner bool `json:"owner,omitempty"`
}

// ListProjectMembers answers with the people holding a direct role, the owner
// included. Readable by anybody who may read the project: knowing who else is
// in a project you are in is not a privilege, and hiding it is how a grant
// stays unnoticed for three weeks.
func (h Handlers) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	item, found, err := h.projectByID(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the project"})
		return
	}
	if !found || !h.hasProjectMembership(callerID, projectID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	members := map[string]*projectMember{}
	if strings.EqualFold(strings.TrimSpace(item.OwnerType), "user") {
		if owner := strings.TrimSpace(item.OwnerID); owner != "" {
			members[strings.ToLower(owner)] = &projectMember{UserID: owner, Role: string(access.RoleAdmin), Owner: true}
		}
	}

	roles, err := h.accessStore.ListProjectRoles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the project members"})
		return
	}
	for _, grant := range roles {
		if !strings.EqualFold(strings.TrimSpace(grant.ProjectID), projectID) {
			continue
		}
		user := strings.TrimSpace(grant.UserID)
		// A revoked grant used to be written as an empty role rather than
		// removed. Those rows exist in installations that ran that code, and
		// listing them would show somebody as a member of nothing.
		if user == "" || strings.TrimSpace(string(grant.Role)) == "" {
			continue
		}
		key := strings.ToLower(user)
		if existing, seen := members[key]; seen {
			existing.Role = string(grant.Role)
			continue
		}
		members[key] = &projectMember{UserID: user, Role: string(grant.Role)}
	}

	items := make([]projectMember, 0, len(members))
	for _, member := range members {
		items = append(items, *member)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Owner != items[j].Owner {
			return items[i].Owner
		}
		return strings.ToLower(items[i].UserID) < strings.ToLower(items[j].UserID)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// DeleteProjectMember takes a role back.
//
// Setting the role to an empty string was the only way, and it left the row
// behind: the person had no access and still appeared to have a grant. This
// removes it, so the list says the truth and an audit of who may reach a
// project can be acted on.
func (h Handlers) DeleteProjectMember(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	userID := strings.TrimSpace(r.PathValue("userID"))
	if projectID == "" || userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and userID path params are required"})
		return
	}
	item, found, err := h.projectByID(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.requireProjectRole(w, projectID, callerID, actionManageMembers, "role management") {
		return
	}
	// The owner's access does not come from this table, so removing a row
	// would change nothing while reporting success. Ownership moves through
	// its own route, which says what it does.
	if h.projectOwnedBy(item, userID) {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "this person owns the project; transfer ownership instead of removing the role",
			"code":  "owner_cannot_be_removed",
		})
		return
	}

	h.accessStore.RemoveRole(projectID, userID)
	h.emitAudit(r, callerID, "project.member.removed", "project", projectID, projectID, "success", "",
		map[string]any{"userId": userID})
	w.WriteHeader(http.StatusNoContent)
}
