package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

// Project roles granted to a team.
//
// The third way a role reaches a project, and deliberately the same shape as
// the organization grant beside it. A project administrator decides which
// teams reach their project; they do not decide who is in those teams, which
// belongs to whoever administers the organization. Splitting it that way is
// what stops a project owner from rewriting an organization's structure to
// widen their own access.

type setProjectTeamRoleRequest struct {
	Role string `json:"role"`
}

type projectTeamRoleView struct {
	TeamID   string `json:"teamId"`
	TeamName string `json:"teamName,omitempty"`
	Role     string `json:"role"`
}

func (h Handlers) ListProjectTeamRoles(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectMember(w, projectID, callerID, "project team roles") {
		return
	}
	// An installation without teams has no grants rather than an error: a
	// screen asking for them should show an empty list, not a failure.
	if h.teamStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []projectTeamRoleView{}})
		return
	}

	grants, err := h.teamStore.ListProjectRoles(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read team roles"})
		return
	}
	items := make([]projectTeamRoleView, 0, len(grants))
	for _, grant := range grants {
		items = append(items, projectTeamRoleView{
			TeamID: grant.TeamID, TeamName: grant.TeamName, Role: string(grant.Role),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) SetProjectTeamRole(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	teamID := strings.TrimSpace(r.PathValue("teamID"))
	if projectID == "" || teamID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project and team are required"})
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
	if !h.requireProjectRole(w, projectID, callerID, actionManageMembers, "team role management") {
		return
	}

	// The team has to exist before it can be granted anything. A grant to an
	// identifier nobody can resolve is a row that opens a project to a group
	// no screen will ever list, and it would survive every later attempt to
	// audit who reaches this project.
	item, found, err := h.teamStore.GetByID(teamID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the team"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "team not found"})
		return
	}

	var req setProjectTeamRoleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	role := access.Role(strings.TrimSpace(req.Role))
	if refusal := h.assignableRoleError(string(role)); refusal != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": refusal})
		return
	}

	if err := h.teamStore.SetProjectRole(projectID, teamID, role); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to grant the role"})
		return
	}
	h.emitAudit(r, callerID, "rbac.team_role.set", "project_team", teamID, projectID,
		"success", "", map[string]any{"role": string(role), "team": item.Name})
	writeJSON(w, http.StatusOK, projectTeamRoleView{
		TeamID: teamID, TeamName: item.Name, Role: string(role),
	})
}

func (h Handlers) DeleteProjectTeamRole(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	teamID := strings.TrimSpace(r.PathValue("teamID"))
	if !h.requireProjectRole(w, projectID, callerID, actionManageMembers, "team role management") {
		return
	}

	if err := h.teamStore.SetProjectRole(projectID, teamID, ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke the role"})
		return
	}
	h.emitAudit(r, callerID, "rbac.team_role.revoked", "project_team", teamID, projectID,
		"success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}
