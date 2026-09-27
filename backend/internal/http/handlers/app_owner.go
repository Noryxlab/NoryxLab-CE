package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handing an application over.
//
// Until ADR-039 an app belonged to whoever launched it and could belong to
// nobody else, so a production went unowned the day that person left. The
// platform's answer was to name a successor at deactivation and move
// everything to another human - which works, and asks somebody to make that
// decision on the worst possible day.
//
// A transfer lets the decision be made in advance and once: the application
// belongs to the organization that paid for it, or to a service account that
// does not resign, and nobody has to inherit it in a hurry.

type appOwnerRequest struct {
	OwnerType string `json:"ownerType"`
	OwnerID   string `json:"ownerId"`
}

func (h Handlers) SetAppOwner(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.appStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "apps are not configured"})
		return
	}
	appID := strings.TrimSpace(r.PathValue("appID"))
	record, found, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the application"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "application not found"})
		return
	}

	// Its current owner, or somebody who administers the project it lives in.
	// Not any member: handing a production to another party is the kind of
	// decision that belongs to whoever answers for the project.
	currentOwner := firstNonEmpty(record.OwnerID, record.OwnerUserID)
	isOwner := isPersonallyOwnedBy(record.OwnerType, currentOwner, callerID)
	if !isOwner && !h.requireProjectRole(w, record.ProjectID, callerID, actionManageMembers, "application ownership") {
		return
	}

	var req appOwnerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	ownerType := strings.ToLower(strings.TrimSpace(req.OwnerType))
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" || (ownerType != "user" && ownerType != "organization") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownerType must be user or organization, with an ownerId"})
		return
	}

	switch ownerType {
	case "organization":
		organization, resolved := h.resolveOrganization(ownerID)
		if !resolved {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no organization named " + ownerID})
			return
		}
		// Only to an organization the caller belongs to. Giving an application
		// away to a party one has nothing to do with is not a handover, it is
		// leaving something in somebody else's house.
		if !h.isGlobalAdminUserID(callerID) && !h.userBelongsToOrganization(callerID, organization.ID) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "you can only hand an application to an organization you belong to",
			})
			return
		}
		ownerID = organization.ID
	case "user":
		// A person or a service account, and in either case somebody the
		// project already reaches: an owner who cannot open the project cannot
		// answer for the application either.
		if !h.hasProjectMembership(ownerID, record.ProjectID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "the new owner must be a member of this project",
			})
			return
		}
	}

	record.OwnerType = ownerType
	record.OwnerID = ownerID
	if err := h.appStore.Upsert(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to change the owner"})
		return
	}
	h.emitAudit(r, callerID, "app.owner.changed", "app", record.ID, record.ProjectID, "success", "",
		map[string]any{"ownerType": ownerType, "ownerId": ownerID, "previousOwner": currentOwner})
	writeJSON(w, http.StatusOK, record)
}
