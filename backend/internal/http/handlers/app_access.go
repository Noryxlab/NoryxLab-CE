package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Changing who can reach a published application.
//
// Access was decided once, at creation, and never again: accessMode and the
// lists beside it were read from the create request and nothing else ever
// wrote them. So an application published for one organization could not be
// opened to a second without deleting it and launching it again - which, on a
// production with a running pod, its URL, its revisions and its usage
// history, is not a thing anybody does to add a colleague.
//
// It surfaced the way these things do: the accounting application was serving,
// an accountant in a newly created organization got a 403, and the answer was
// "there is no screen for that". Applications are services and are never
// reaped (ADR); it follows that everything about them has to be changeable
// while they run.
//
// The decision belongs to the application's owner or to somebody who
// administers its project - the same bar as handing the application over,
// because widening who may open a production is the same kind of decision.
type appAccessRequest struct {
	AccessMode           string   `json:"accessMode"`
	AllowedUsers         []string `json:"allowedUsers"`
	AllowedOrganizations []string `json:"allowedOrganizations"`
}

func (h Handlers) SetAppAccess(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.appStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "apps are not configured"})
		return
	}
	record, found, err := h.appStore.GetByID(strings.TrimSpace(r.PathValue("appID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the application"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "application not found"})
		return
	}
	owner := firstNonEmpty(record.OwnerID, record.OwnerUserID)
	if !isPersonallyOwnedBy(record.OwnerType, owner, callerID) &&
		!h.requireProjectRole(w, record.ProjectID, callerID, actionManageMembers, "application access") {
		return
	}

	var req appAccessRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	mode, users, organizations, valid := h.resolveAppAccess(w, record.ProjectID, record.Kind,
		req.AccessMode, req.AllowedUsers, req.AllowedOrganizations)
	if !valid {
		return
	}

	previous := map[string]any{
		"accessMode":    record.AccessMode,
		"organizations": record.AllowedOrganizations,
		"users":         record.AllowedUsers,
	}
	record.AccessMode = mode
	record.AllowedUsers = users
	record.AllowedOrganizations = organizations
	if err := h.appStore.Upsert(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to change the access"})
		return
	}
	// Audited with both sides. "access changed" answers nothing six months
	// later; who could open it before and who can now is the question.
	h.emitAudit(r, callerID, "app.access.changed", "app", record.ID, record.ProjectID, "success", "",
		map[string]any{
			"accessMode":    mode,
			"organizations": organizations,
			"users":         users,
			"previous":      previous,
		})
	writeJSON(w, http.StatusOK, record)
}

// resolveAppAccess checks an access declaration and returns it normalised.
//
// Shared by creation and by the change above, because a rule enforced in one
// place and not the other is how an application ends up in a state the create
// path would have refused. The regulated refusal in particular: a project that
// attaches regulated data cannot serve an application publicly, and that must
// hold the day somebody edits the access as much as the day they chose it.
func (h Handlers) resolveAppAccess(
	w http.ResponseWriter, projectID, kind, mode string, users, organizations []string,
) (string, []string, []string, bool) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if kind == "dashboard" {
		// A dashboard reads the project it belongs to; its audience is that
		// project and choosing another is meaningless rather than refused.
		return "project", nil, nil, true
	}
	if mode == "" {
		mode = "private"
	}
	// Exactly the set the create path accepted, no wider. "project" is a
	// dashboard's mode, decided above rather than chosen, and letting an
	// ordinary application take it here would have widened what is allowed
	// as a side effect of sharing this function - which is the opposite of
	// why it is shared.
	switch mode {
	case "private", "users", "organization", publicAppAccessMode:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "accessMode must be public, organization, users, or private"})
		return "", nil, nil, false
	}

	if mode == publicAppAccessMode {
		if name := h.projectHoldsRegulatedDataNamed(projectID); name != "" {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "this project attaches regulated data (" + name + "): an application here cannot be served publicly",
			})
			return "", nil, nil, false
		}
	}
	if mode == "organization" {
		if len(organizations) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "allowedOrganizations is required for organization access"})
			return "", nil, nil, false
		}
		resolved := make([]string, 0, len(organizations))
		for _, identifier := range organizations {
			organization, found := h.resolveOrganization(identifier)
			if !found {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "no organization named " + strings.TrimSpace(identifier)})
				return "", nil, nil, false
			}
			resolved = append(resolved, organization.ID)
		}
		organizations = resolved
	}
	if mode == "users" && len(users) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "allowedUsers is required for users access"})
		return "", nil, nil, false
	}
	return mode, normalizeAppSubjects(users), normalizeAppSubjects(organizations), true
}
