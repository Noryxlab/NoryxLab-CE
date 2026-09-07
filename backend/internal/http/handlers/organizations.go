package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

// ListMyOrganizations answers with the organizations the caller belongs to,
// as the identity provider holds them right now.
//
// The interface used to read this from the token, which meant it depended on a
// realm binding the `organization` client scope as a default rather than an
// optional one. Where that binding was missing - and it was missing on both
// production installations - every user saw a platform where they belonged to
// nothing, while Keycloak held the memberships all along. It also meant a
// membership granted this morning only appeared after the person signed out
// and back in.
//
// Asking the platform is the fix: it holds administrator credentials on the
// realm and can simply look.
func (h Handlers) ListMyOrganizations(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	items, err := h.keycloak.ListUserOrganizations(identity.UserID())
	if err != nil {
		// Silent on purpose, like the other identity lookups: an unreachable
		// directory is an outage, and reporting it as "you belong to no
		// organization" would send somebody looking for a membership that is
		// there.
		log.Printf("organizations: cannot read the memberships of %q: %v", identity.UserID(), err)
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
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

// Naming the members is the difference between a refusal an administrator can
// act on and one they have to investigate. A disabled account still counts as
// a member, and that is exactly the case that looks like a bug: the account
// was deactivated, so it no longer appears on screens that hide disabled
// users, and it still blocks the organization from being deleted.
func memberLabels(members []keycloak.User) []string {
	names := make([]string, 0, len(members))
	for _, member := range members {
		label := strings.TrimSpace(member.Username)
		if label == "" {
			label = strings.TrimSpace(member.Email)
		}
		if label == "" {
			label = member.ID
		}
		if !member.Enabled {
			label += " (disabled)"
		}
		names = append(names, label)
	}
	return names
}

func blockingMembersMessage(names []string) string {
	shown := names
	if len(shown) > 5 {
		shown = append(shown[:5:5], fmt.Sprintf("and %d more", len(names)-5))
	}
	return "the organization still has " + strconv.Itoa(len(names)) + " member(s): " +
		strings.Join(shown, ", ") + ". Remove them from the organization first; " +
		"a disabled account is still a member."
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
		names := memberLabels(members)
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   blockingMembersMessage(names),
			"members": names,
		})
		return
	}
	projects, err := h.projectStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify organization-owned projects"})
		return
	}
	for _, item := range projects {
		if strings.EqualFold(item.OwnerType, "organization") && strings.EqualFold(item.OwnerID, organizationID) {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "the organization still owns the project " + item.Name +
					". Transfer it to another owner first.",
			})
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
