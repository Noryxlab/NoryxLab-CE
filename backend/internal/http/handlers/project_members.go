package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

type setProjectMemberRoleRequest struct {
	Role string `json:"role"`
}

type inviteProjectMemberRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

func (h Handlers) SetProjectMemberRole(w http.ResponseWriter, r *http.Request) {
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

	exists, err := h.projectExists(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if !h.requireProjectRole(w, projectID, callerID, func(role access.Role) bool { return role == access.RoleAdmin }, "role management") {
		return
	}

	var req setProjectMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	h.accessStore.SetRole(projectID, userID, role)
	writeJSON(w, http.StatusOK, map[string]string{
		"projectId": projectID,
		"userId":    userID,
		"role":      string(role),
	})
}

func (h Handlers) InviteProjectMember(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID path param is required"})
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

	if !h.requireProjectRole(w, projectID, callerID, func(role access.Role) bool { return role == access.RoleAdmin }, "project invitation") {
		return
	}

	var req inviteProjectMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Role = strings.TrimSpace(req.Role)
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}
	if req.Role == "" {
		req.Role = string(access.RoleEditor)
	}
	role := access.Role(req.Role)
	switch role {
	case access.RoleViewer, access.RoleEditor, access.RoleAdmin:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be viewer, editor or admin"})
		return
	}

	h.accessStore.SetRole(projectID, req.UserID, role)
	writeJSON(w, http.StatusCreated, map[string]string{
		"projectId": projectID,
		"userId":    req.UserID,
		"role":      string(role),
		"status":    "invited",
	})
}
