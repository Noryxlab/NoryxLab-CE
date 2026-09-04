package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// writeKeycloakError answers an identity-provider failure.
//
// A 404 from Keycloak means the organization does not exist and the caller
// mistyped an identifier; everything else means the identity provider is
// unreachable or refusing us. Reporting both as 502 sent an operator looking
// for a Keycloak outage that was not happening.
//
// Keycloak's own response body goes to the log and never to the caller: it is
// another system's internals, and nobody on the far side can act on it.
func (h Handlers) writeKeycloakError(w http.ResponseWriter, action string, err error) {
	log.Printf("keycloak: %s: %v", action, err)
	if keycloak.IsNotFound(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such organization"})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to " + action})
}

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
		h.writeKeycloakError(w, "fetch organizations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) ListAvailableOrganizations(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentity(w, r); !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	items, err := h.keycloak.ListOrganizations()
	if err != nil {
		h.writeKeycloakError(w, "fetch organizations", err)
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
		h.writeKeycloakError(w, "create organization", err)
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
		h.writeKeycloakError(w, "verify organization members", err)
		return
	}
	if len(members) > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "organization must have no members before deletion"})
		return
	}
	projects, err := h.projectStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify organization-owned projects"})
		return
	}
	for _, item := range projects {
		if strings.EqualFold(item.OwnerType, "organization") && strings.EqualFold(item.OwnerID, organizationID) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "organization must own no projects before deletion"})
			return
		}
	}
	if err := h.keycloak.DeleteOrganization(organizationID); err != nil {
		h.writeKeycloakError(w, "delete organization", err)
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
		h.writeKeycloakError(w, "fetch organization members", err)
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
		h.writeKeycloakError(w, "add organization member", err)
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
		h.writeKeycloakError(w, "remove organization member", err)
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "organization.member.remove", "organization", organizationID, "", "success", "", map[string]any{"userId": userID})
	w.WriteHeader(http.StatusNoContent)
}
