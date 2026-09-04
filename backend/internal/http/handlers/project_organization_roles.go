package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

// Project roles granted to an organization.
//
// Assigning people one at a time is workable for a team of five and a reason
// not to buy at fifty: an administrator adding a researcher has to remember
// every project that person should reach, and someone leaving has to be
// removed from each one by hand - which is how stale access accumulates.
//
// A grant here follows the organization's membership in Keycloak, which owns
// who belongs where. Noryx owns only what belonging permits.

type setProjectOrganizationRoleRequest struct {
	Role string `json:"role"`
}

type projectOrganizationRoleView struct {
	OrganizationID   string `json:"organizationId"`
	OrganizationName string `json:"organizationName,omitempty"`
	Role             string `json:"role"`
}

func (h Handlers) ListProjectOrganizationRoles(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectMember(w, projectID, callerID, "project organization roles") {
		return
	}

	grants, err := h.accessStore.ListOrganizationRoles(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read organization roles"})
		return
	}

	// Names are resolved for display only. A grant survives an unreachable
	// directory: showing an identifier is worse than showing a name and far
	// better than pretending the grant is gone.
	names := map[string]string{}
	if h.keycloak != nil {
		if organizations, err := h.keycloak.ListOrganizations(); err == nil {
			for _, organization := range organizations {
				names[strings.TrimSpace(organization.ID)] = organization.Name
			}
		}
	}

	items := make([]projectOrganizationRoleView, 0, len(grants))
	for _, grant := range grants {
		items = append(items, projectOrganizationRoleView{
			OrganizationID:   grant.OrganizationID,
			OrganizationName: names[strings.TrimSpace(grant.OrganizationID)],
			Role:             string(grant.Role),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) SetProjectOrganizationRole(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	if projectID == "" || organizationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project and organization are required"})
		return
	}

	exists, err := h.projectExists(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.requireProjectRole(w, projectID, callerID, actionManageMembers, "organization role management") {
		return
	}

	var req setProjectOrganizationRoleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	role := access.Role(strings.TrimSpace(req.Role))
	switch role {
	case access.RoleViewer, access.RoleEditor, access.RoleAdmin:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be viewer, editor or admin"})
		return
	}

	if err := h.accessStore.SetOrganizationRole(projectID, organizationID, role); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to grant the role"})
		return
	}
	h.emitAudit(r, callerID, "rbac.organization_role.set", "project_organization", organizationID, projectID,
		"success", "", map[string]any{"role": string(role)})
	writeJSON(w, http.StatusOK, projectOrganizationRoleView{
		OrganizationID: organizationID, Role: string(role),
	})
}

func (h Handlers) DeleteProjectOrganizationRole(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	if !h.requireProjectRole(w, projectID, callerID, actionManageMembers, "organization role management") {
		return
	}

	if err := h.accessStore.SetOrganizationRole(projectID, organizationID, ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke the role"})
		return
	}
	h.emitAudit(r, callerID, "rbac.organization_role.revoked", "project_organization", organizationID, projectID,
		"success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}
