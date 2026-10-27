package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
)

// What a role may create (ADR-049).
//
// Functions in rows, roles in columns, yes or no in the cells. The handlers
// that create something ask one question here - "may this role do this" - in
// place of the rules that used to be scattered: registering regulated data
// hard-coded on the realm role, project creation open to anybody who could
// sign in, and nothing on any screen saying either.
//
// Who holds which row: the realm role makes an administrator and is never
// assigned here; an assignment names a shipped or custom row; nobody assigned
// means the user row, which is what every account could already do.

// platformRoleOf resolves the row this caller holds.
func (h Handlers) platformRoleOf(identity auth.Identity) platformrole.Role {
	if h.isGlobalAdmin(identity) {
		role, _ := platformrole.ShippedByKey(platformrole.KeyAdministrator)
		return role
	}
	user, _ := platformrole.ShippedByKey(platformrole.KeyUser)
	if h.platformRoleStore == nil {
		return user
	}
	for _, candidate := range identityIdentifiers(identity) {
		if candidate == "" {
			continue
		}
		assignment, found, err := h.platformRoleStore.GetAssignment(candidate)
		if err != nil || !found {
			continue
		}
		if shipped, ok := platformrole.ShippedByKey(assignment.RoleKey); ok && shipped.Assignable {
			return shipped
		}
		if custom, ok, err := h.platformRoleStore.GetCustom(assignment.RoleKey); err == nil && ok {
			return custom
		}
		// A row that no longer exists falls back to the user row rather than
		// to nothing: deleting a custom role must not lock its holders out.
		return user
	}
	return user
}

// can answers the matrix for one capability.
func (h Handlers) can(identity auth.Identity, capability platformrole.Capability) bool {
	return h.platformRoleOf(identity).Has(capability)
}

// requireCapability refuses with the reason, naming the role and the function,
// so the person reads what to ask for rather than "forbidden".
func (h Handlers) requireCapability(w http.ResponseWriter, identity auth.Identity, capability platformrole.Capability) bool {
	role := h.platformRoleOf(identity)
	if role.Has(capability) {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error":      "your role (" + role.Name + ") does not allow " + string(capability) + "; ask a platform administrator",
		"role":       role.Key,
		"capability": string(capability),
	})
	return false
}

// allRoles is the shipped rows followed by the installation's, with "held by"
// counted from the assignments and the directory.
func (h Handlers) allRoles() ([]platformrole.Role, map[string]int, error) {
	roles := platformrole.Shipped()
	counts := map[string]int{}
	if h.platformRoleStore != nil {
		custom, err := h.platformRoleStore.ListCustom()
		if err != nil {
			return nil, nil, err
		}
		roles = append(roles, custom...)
		assignments, err := h.platformRoleStore.ListAssignments()
		if err != nil {
			return nil, nil, err
		}
		for _, assignment := range assignments {
			counts[assignment.RoleKey]++
		}
	}
	// The administrator row is held by whoever the directory says.
	if h.keycloak != nil {
		if members, err := h.keycloak.ListRealmRoleMembers(globalAdminRole); err == nil {
			counts[platformrole.KeyAdministrator] = len(members)
		}
	}
	return roles, counts, nil
}

type platformRoleView struct {
	platformrole.Role
	HeldBy int `json:"heldBy"`
}

// GetMyCapabilities answers "what may I do", for an interface that hides what
// the caller may not do and never re-implements the rule.
func (h Handlers) GetMyCapabilities(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	role := h.platformRoleOf(identity)
	capabilities := map[string]bool{}
	for _, capability := range platformrole.All {
		capabilities[string(capability)] = role.Has(capability)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"role":         map[string]string{"key": role.Key, "name": role.Name},
		"capabilities": capabilities,
	})
}

// ListPlatformRoles is the matrix as a document.
func (h Handlers) ListPlatformRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "rbac policy"); !ok {
		return
	}
	roles, counts, err := h.allRoles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the platform roles"})
		return
	}
	views := make([]platformRoleView, 0, len(roles))
	for _, role := range roles {
		views = append(views, platformRoleView{Role: role, HeldBy: counts[role.Key]})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"capabilities": platformrole.All,
		"roles":        views,
	})
}

type platformRoleRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

// UpsertPlatformRole writes a custom row. A shipped key is refused: a locked
// row describes what the platform does and is not a setting.
func (h Handlers) UpsertPlatformRole(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "rbac policy")
	if !ok {
		return
	}
	if h.platformRoleStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "platform roles require a persistent store"})
		return
	}
	key := strings.ToLower(strings.TrimSpace(r.PathValue("roleKey")))
	if _, shipped := platformrole.ShippedByKey(key); shipped {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "role " + key + " ships with the platform and cannot be changed; add a role of your own instead"})
		return
	}
	if key == "" || strings.ContainsAny(key, " /\\") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a role key is one word, lowercase"})
		return
	}
	var req platformRoleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	for _, capability := range req.Capabilities {
		if !platformrole.Known(capability) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown capability " + capability})
			return
		}
	}
	capabilities := make([]platformrole.Capability, 0, len(req.Capabilities))
	for _, capability := range req.Capabilities {
		capabilities = append(capabilities, platformrole.Capability(capability))
	}
	role := platformrole.Role{Key: key, Name: req.Name, Description: req.Description, Capabilities: capabilities, Assignable: true}
	role.Normalise()
	if err := h.platformRoleStore.UpsertCustom(role); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save the role"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "platform_role.saved", "platform_role", role.Key, "", "success", "",
		map[string]any{"name": role.Name, "capabilities": role.Capabilities})
	saved, _, _ := h.platformRoleStore.GetCustom(role.Key)
	writeJSON(w, http.StatusOK, saved)
}

// DeletePlatformRole removes a custom row. Refused while somebody holds it:
// a role that vanishes under its holders is a lockout nobody decided.
func (h Handlers) DeletePlatformRole(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "rbac policy")
	if !ok {
		return
	}
	if h.platformRoleStore == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	key := strings.ToLower(strings.TrimSpace(r.PathValue("roleKey")))
	if _, shipped := platformrole.ShippedByKey(key); shipped {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "role " + key + " ships with the platform and cannot be removed"})
		return
	}
	assignments, err := h.platformRoleStore.ListAssignments()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the assignments"})
		return
	}
	holders := 0
	for _, assignment := range assignments {
		if assignment.RoleKey == key {
			holders++
		}
	}
	if holders > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "role " + key + " is still held by " + itoa(holders) + " account(s); reassign them first"})
		return
	}
	if err := h.platformRoleStore.DeleteCustom(key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove the role"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "platform_role.removed", "platform_role", key, "", "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// GetUserPlatformRole says which row one person holds, and whether it was
// assigned or is the default.
func (h Handlers) GetUserPlatformRole(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "users"); !ok {
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	role := h.platformRoleOf(auth.Identity{Username: userID, Subject: userID})
	assigned := false
	if h.platformRoleStore != nil {
		_, assigned, _ = h.platformRoleStore.GetAssignment(userID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"userId": userID, "role": role, "assigned": assigned})
}

type assignRoleRequest struct {
	RoleKey string `json:"roleKey"`
}

// SetUserPlatformRole assigns a row to a person - at onboarding, or later
// without recreating the account.
func (h Handlers) SetUserPlatformRole(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.platformRoleStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "platform roles require a persistent store"})
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	var req assignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RoleKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "roleKey is required"})
		return
	}
	key := strings.ToLower(strings.TrimSpace(req.RoleKey))
	if shipped, ok := platformrole.ShippedByKey(key); ok {
		if !shipped.Assignable {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "the administrator role is the directory's realm role and is not assigned here"})
			return
		}
	} else if _, ok, err := h.platformRoleStore.GetCustom(key); err != nil || !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no role named " + key})
		return
	}
	item := platformrole.Assignment{UserID: userID, RoleKey: key, AssignedBy: identity.UserID(), AssignedAt: time.Now().UTC()}
	if err := h.platformRoleStore.SetAssignment(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to assign the role"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "platform_role.assigned", "user", userID, "", "success", "",
		map[string]any{"roleKey": key})
	writeJSON(w, http.StatusOK, item)
}

// ClearUserPlatformRole puts the person back on the user row.
func (h Handlers) ClearUserPlatformRole(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	if h.platformRoleStore != nil {
		if err := h.platformRoleStore.ClearAssignment(userID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear the role"})
			return
		}
	}
	h.emitAdvancedAudit(r, identity.UserID(), "platform_role.cleared", "user", userID, "", "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}
