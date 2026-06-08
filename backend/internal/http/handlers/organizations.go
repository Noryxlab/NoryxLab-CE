package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (h Handlers) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "organizations"); !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	items, err := h.keycloak.ListOrganizations()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to fetch organizations from keycloak: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Alias string `json:"alias"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Alias) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization name and alias are required"})
		return
	}
	item, err := h.keycloak.CreateOrganization(req.Name, req.Alias)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to create organization: " + err.Error()})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.create", "organization", item.ID, "", "success", "", map[string]any{"name": item.Name, "alias": item.Alias})
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	if organizationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization id is required"})
		return
	}
	members, err := h.keycloak.ListOrganizationMembers(organizationID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to verify organization members: " + err.Error()})
		return
	}
	if len(members) > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "organization must have no members before deletion"})
		return
	}
	if err := h.keycloak.DeleteOrganization(organizationID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to delete organization: " + err.Error()})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.delete", "organization", organizationID, "", "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) ListOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "organizations"); !ok {
		return
	}
	items, err := h.keycloak.ListOrganizationMembers(r.PathValue("organizationID"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to fetch organization members: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AddOrganizationMember(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	userID := strings.TrimSpace(r.PathValue("userID"))
	if err := h.keycloak.AddOrganizationMember(organizationID, userID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to add organization member: " + err.Error()})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.member.add", "organization", organizationID, "", "success", "", map[string]any{"userId": userID})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) RemoveOrganizationMember(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "organizations")
	if !ok {
		return
	}
	organizationID := strings.TrimSpace(r.PathValue("organizationID"))
	userID := strings.TrimSpace(r.PathValue("userID"))
	if err := h.keycloak.RemoveOrganizationMember(organizationID, userID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to remove organization member: " + err.Error()})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.member.remove", "organization", organizationID, "", "success", "", map[string]any{"userId": userID})
	w.WriteHeader(http.StatusNoContent)
}
