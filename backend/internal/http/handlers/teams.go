package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/google/uuid"
)

// Teams, and who is in them.
//
// A team is a subdivision of an organization, so it is administered where
// organizations are: the same admin module decides both. The split that
// matters is the other one - an organization administrator says who is in
// which team, and a project administrator says which teams reach their
// project. Neither can do the other's half, which is what keeps a team lead
// from granting themselves a project and a project owner from rewriting an
// organization's structure.

type teamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type teamMemberRequest struct {
	UserID string `json:"userId"`
}

// teamStoreReady answers the one question every handler here has to ask first.
//
// The store is optional so the platform runs unchanged without it, which means
// every entry point has to say so rather than panicking on a nil interface.
func (h Handlers) teamStoreReady(w http.ResponseWriter) bool {
	if h.teamStore == nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "teams are not enabled on this installation"})
		return false
	}
	return true
}

func (h Handlers) ListTeams(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "organizations"); !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	items, err := h.teamStore.ListByOrganization(organizationID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read teams"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateTeam(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))

	var req teamRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	now := time.Now().UTC()
	item := team.Team{
		ID: uuid.NewString(), OrganizationID: organizationID,
		Name: req.Name, Description: req.Description,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := item.Normalise(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.teamStore.Create(item); err != nil {
		h.writeTeamStoreError(w, err, "failed to create the team")
		return
	}
	h.emitAudit(r, identity.Username, "team.created", "team", item.ID, "",
		"success", "", map[string]any{"name": item.Name, "organizationId": organizationID})
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	existing, found := h.loadTeam(w, r)
	if !found {
		return
	}

	var req teamRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	// The organization is carried over rather than read from the request: a
	// rename must not be able to move a team between organizations, which
	// would carry its grants with it to people who were never meant to have
	// them.
	updated := team.Team{
		ID: existing.ID, OrganizationID: existing.OrganizationID,
		Name: req.Name, Description: req.Description,
		CreatedAt: existing.CreatedAt, UpdatedAt: time.Now().UTC(),
	}
	if err := updated.Normalise(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.teamStore.Update(updated); err != nil {
		h.writeTeamStoreError(w, err, "failed to update the team")
		return
	}
	h.emitAudit(r, identity.Username, "team.updated", "team", updated.ID, "",
		"success", "", map[string]any{"name": updated.Name})
	writeJSON(w, http.StatusOK, updated)
}

// DeleteTeam removes the team, its membership and its grants.
//
// The audit line records how many people and how many projects were affected,
// because "team deleted" is not an answer to the question somebody will ask
// afterwards, which is who lost what.
func (h Handlers) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	existing, found := h.loadTeam(w, r)
	if !found {
		return
	}

	members, _ := h.teamStore.ListMembers(existing.ID)
	if err := h.teamStore.Delete(existing.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete the team"})
		return
	}
	h.emitAudit(r, identity.Username, "team.deleted", "team", existing.ID, "",
		"success", "", map[string]any{"name": existing.Name, "members": len(members)})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) ListTeamMembers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "organizations"); !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	existing, found := h.loadTeam(w, r)
	if !found {
		return
	}
	members, err := h.teamStore.ListMembers(existing.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read members"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": members})
}

func (h Handlers) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	existing, found := h.loadTeam(w, r)
	if !found {
		return
	}

	var req teamMemberRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}
	if err := h.teamStore.AddMember(existing.ID, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to add the member"})
		return
	}
	h.emitAudit(r, identity.Username, "team.member.added", "team", existing.ID, "",
		"success", "", map[string]any{"userId": userID, "team": existing.Name})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	if !h.teamStoreReady(w) {
		return
	}
	existing, found := h.loadTeam(w, r)
	if !found {
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	if err := h.teamStore.RemoveMember(existing.ID, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove the member"})
		return
	}
	// Recorded because this is the line an auditor reads to explain why
	// somebody stopped being able to reach a project.
	h.emitAudit(r, identity.Username, "team.member.removed", "team", existing.ID, "",
		"success", "", map[string]any{"userId": userID, "team": existing.Name})
	w.WriteHeader(http.StatusNoContent)
}

// ListMyTeams is what a person can ask about themselves.
//
// Not behind the admin module: a developer looking at a project they cannot
// reach needs to know which team would have given it to them, and sending them
// to an administrator to find out wastes two people.
func (h Handlers) ListMyTeams(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.teamStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []team.Team{}})
		return
	}
	items, err := h.teamStore.ListByUser(callerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read teams"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// loadTeam resolves the team named in the path, answering 404 itself.
func (h Handlers) loadTeam(w http.ResponseWriter, r *http.Request) (team.Team, bool) {
	teamID := strings.TrimSpace(r.PathValue("teamID"))
	if teamID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a team is required"})
		return team.Team{}, false
	}
	item, found, err := h.teamStore.GetByID(teamID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the team"})
		return team.Team{}, false
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "team not found"})
		return team.Team{}, false
	}
	return item, true
}

// writeTeamStoreError maps the domain's refusals onto status codes.
//
// A duplicate name is the caller's mistake and a 409 they can act on; anything
// else is ours and stays a 500 with a message that does not leak the database.
func (h Handlers) writeTeamStoreError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, team.ErrNameTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, team.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fallback})
	}
}
