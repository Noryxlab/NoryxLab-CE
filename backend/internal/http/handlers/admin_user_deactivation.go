package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// Turning an account off, and handing on what it owned.
//
// Deleting a user was the only option, and it is the wrong one: it breaks the
// audit trail, which stops being attributable the moment the actor no longer
// exists. Disabling stops access immediately and keeps the record.
//
// The part that is not optional is what the person owned. A project whose owner
// is gone has nobody who can grant access to it, an app nobody can republish, a
// data source nobody can rotate. So a deactivation names a successor, and the
// platform hands everything over in the same operation - or refuses, and says
// what would have been orphaned.
//
// Two things are deliberately *not* transferred:
//
//   - personal API tokens, which are revoked. A token acts as its owner, so an
//     account disabled while its tokens still work is not disabled at all.
//   - personal secrets, which belong to the person and not to the role. They
//     stay with the disabled account, and are listed in the answer so an
//     administrator knows what did not move.

type deactivateUserRequest struct {
	// SuccessorUserID receives everything the account owned. Required when it
	// owns anything at all.
	SuccessorUserID string `json:"successorUserId"`
}

type transferReport struct {
	Projects     []string `json:"projects,omitempty"`
	Datasets     []string `json:"datasets,omitempty"`
	Ontologies   []string `json:"ontologies,omitempty"`
	Apps         []string `json:"apps,omitempty"`
	Datasources  []string `json:"datasources,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
}

func (t transferReport) count() int {
	return len(t.Projects) + len(t.Datasets) + len(t.Ontologies) + len(t.Apps) + len(t.Datasources) + len(t.Repositories)
}

func (h Handlers) DeactivateUserAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	target := strings.TrimSpace(r.PathValue("userID"))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}
	// An administrator disabling themselves locks the platform's own
	// administration behind an account that can no longer sign in.
	if strings.EqualFold(target, identity.UserID()) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "you cannot disable your own account"})
		return
	}

	var req deactivateUserRequest
	if r.Body != nil && r.ContentLength != 0 {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a valid successor is required"})
			return
		}
	}
	successor := strings.TrimSpace(req.SuccessorUserID)

	owned, err := h.ownedBy(target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read what this account owns"})
		return
	}
	if owned.count() > 0 && successor == "" {
		// Named rather than counted: an administrator deciding who inherits a
		// project needs to know which project.
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": fmt.Sprintf("this account owns %d resource(s); name a successor to receive them", owned.count()),
			"code":  "successor_required",
			"owns":  owned,
		})
		return
	}
	if successor != "" && strings.EqualFold(successor, target) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the successor cannot be the account being disabled"})
		return
	}

	transferred := transferReport{}
	if successor != "" && owned.count() > 0 {
		transferred, err = h.transferOwnership(target, successor)
		if err != nil {
			// Deliberately before disabling: a half-transfer that also locked
			// the owner out would leave resources nobody can reach.
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "transfer failed, the account was left enabled: " + err.Error()})
			return
		}
	}

	revoked := h.revokeAllTokens(target)

	if err := h.keycloak.SetUserEnabled(target, false); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider refused to disable the account: " + err.Error()})
		return
	}

	h.emitAudit(r, identity.UserID(), "user.disable", "user", target, "", "success", "", map[string]any{
		"successor":       successor,
		"transferred":     transferred.count(),
		"tokensRevoked":   revoked,
		"secretsLeftWith": target,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"disabled":      target,
		"successor":     successor,
		"transferred":   transferred,
		"tokensRevoked": revoked,
		// Said out loud rather than left to be discovered: personal secrets do
		// not move, because they belong to the person and not to the role.
		"note": "personal secrets were not transferred; they stay with the disabled account",
	})
}

func (h Handlers) ReactivateUserAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	target := strings.TrimSpace(r.PathValue("userID"))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}
	if err := h.keycloak.SetUserEnabled(target, true); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider refused to enable the account: " + err.Error()})
		return
	}
	// What was transferred stays transferred, and the tokens stay revoked:
	// re-enabling gives back the ability to sign in, not the ownership somebody
	// else has been carrying since.
	h.emitAudit(r, identity.UserID(), "user.enable", "user", target, "", "success", "", nil)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": target})
}

// ownedBy lists what an account owns personally. Resources owned by an
// organization are not included: they already have somebody else responsible
// for them, which is the point of organization ownership.
func (h Handlers) ownedBy(userID string) (transferReport, error) {
	report := transferReport{}
	userID = strings.TrimSpace(userID)

	if h.projectStore != nil {
		projects, err := h.projectStore.List()
		if err != nil {
			return report, err
		}
		for _, item := range projects {
			if isPersonallyOwnedBy(item.OwnerType, item.OwnerID, userID) {
				report.Projects = append(report.Projects, item.Name)
			}
		}
	}
	if h.datasetStore != nil {
		datasets, err := h.datasetStore.ListAll()
		if err != nil {
			return report, err
		}
		for _, item := range datasets {
			if isPersonallyOwnedBy(item.OwnerType, item.OwnerID, userID) || strings.EqualFold(item.OwnerUserID, userID) {
				report.Datasets = append(report.Datasets, item.Name)
			}
		}
	}
	if h.ontologyStore != nil {
		ontologies, err := h.ontologyStore.ListAll()
		if err != nil {
			return report, err
		}
		for _, item := range ontologies {
			if isPersonallyOwnedBy(item.OwnerType, item.OwnerID, userID) || strings.EqualFold(item.OwnerUserID, userID) {
				report.Ontologies = append(report.Ontologies, item.Name)
			}
		}
	}
	if h.appStore != nil {
		apps, err := h.appStore.List()
		if err != nil {
			return report, err
		}
		for _, item := range apps {
			if strings.EqualFold(strings.TrimSpace(item.OwnerUserID), userID) {
				report.Apps = append(report.Apps, item.Name)
			}
		}
	}
	if h.datasourceStore != nil {
		datasources, err := h.datasourceStore.ListAll()
		if err != nil {
			return report, err
		}
		for _, item := range datasources {
			if strings.EqualFold(strings.TrimSpace(item.OwnerUserID), userID) {
				report.Datasources = append(report.Datasources, item.Name)
			}
		}
	}
	if h.repositoryStore != nil {
		repositories, err := h.repositoryStore.ListAll()
		if err != nil {
			return report, err
		}
		for _, item := range repositories {
			if strings.EqualFold(strings.TrimSpace(item.OwnerUserID), userID) {
				report.Repositories = append(report.Repositories, item.Name)
			}
		}
	}
	return report, nil
}

func isPersonallyOwnedBy(ownerType, ownerID, userID string) bool {
	kind := strings.ToLower(strings.TrimSpace(ownerType))
	if kind == "" {
		kind = "user"
	}
	return kind == "user" && strings.EqualFold(strings.TrimSpace(ownerID), userID)
}

func (h Handlers) transferOwnership(from, to string) (transferReport, error) {
	moved := transferReport{}

	projects, err := h.projectStore.List()
	if err != nil {
		return moved, err
	}
	for _, item := range projects {
		if !isPersonallyOwnedBy(item.OwnerType, item.OwnerID, from) {
			continue
		}
		if err := h.projectStore.UpdateOwner(item.ID, "user", to); err != nil {
			return moved, err
		}
		moved.Projects = append(moved.Projects, item.Name)
	}

	if h.datasetStore != nil {
		datasets, err := h.datasetStore.ListAll()
		if err != nil {
			return moved, err
		}
		for _, item := range datasets {
			if !isPersonallyOwnedBy(item.OwnerType, item.OwnerID, from) && !strings.EqualFold(item.OwnerUserID, from) {
				continue
			}
			if err := h.datasetStore.UpdateOwner(item.ID, "user", to); err != nil {
				return moved, err
			}
			moved.Datasets = append(moved.Datasets, item.Name)
		}
	}

	if h.ontologyStore != nil {
		ontologies, err := h.ontologyStore.ListAll()
		if err != nil {
			return moved, err
		}
		for _, item := range ontologies {
			if !isPersonallyOwnedBy(item.OwnerType, item.OwnerID, from) && !strings.EqualFold(item.OwnerUserID, from) {
				continue
			}
			if err := h.ontologyStore.UpdateOwner(item.ID, "user", to); err != nil {
				return moved, err
			}
			moved.Ontologies = append(moved.Ontologies, item.Name)
		}
	}

	if h.appStore != nil {
		apps, err := h.appStore.List()
		if err != nil {
			return moved, err
		}
		for _, item := range apps {
			if !strings.EqualFold(strings.TrimSpace(item.OwnerUserID), from) {
				continue
			}
			item.OwnerUserID = to
			if err := h.appStore.Upsert(item); err != nil {
				return moved, err
			}
			moved.Apps = append(moved.Apps, item.Name)
		}
	}

	if h.datasourceStore != nil {
		datasources, err := h.datasourceStore.ListAll()
		if err != nil {
			return moved, err
		}
		for _, item := range datasources {
			if !strings.EqualFold(strings.TrimSpace(item.OwnerUserID), from) {
				continue
			}
			item.OwnerUserID = to
			if err := h.datasourceStore.Upsert(item); err != nil {
				return moved, err
			}
			moved.Datasources = append(moved.Datasources, item.Name)
		}
	}

	if h.repositoryStore != nil {
		repositories, err := h.repositoryStore.ListAll()
		if err != nil {
			return moved, err
		}
		for _, item := range repositories {
			if !strings.EqualFold(strings.TrimSpace(item.OwnerUserID), from) {
				continue
			}
			item.OwnerUserID = to
			if err := h.repositoryStore.Update(item); err != nil {
				return moved, err
			}
			moved.Repositories = append(moved.Repositories, item.Name)
		}
	}

	return moved, nil
}

// revokeAllTokens returns how many were still active. A disabled account whose
// tokens keep working is not disabled; this is the half of the operation that
// has nothing to do with Keycloak.
func (h Handlers) revokeAllTokens(userID string) int {
	if h.apiTokenStore == nil {
		return 0
	}
	tokens, err := h.apiTokenStore.ListByUser(userID)
	if err != nil {
		return 0
	}
	now := time.Now().UTC()
	revoked := 0
	for _, token := range tokens {
		if !token.Active(now) {
			continue
		}
		if ok, err := h.apiTokenStore.Revoke(token.ID, userID, now); err == nil && ok {
			revoked++
		}
	}
	return revoked
}

// GetUserOwnedResources answers what an account owns, so the interface can ask
// for a successor before the administrator commits to anything - rather than
// after a refusal.
func (h Handlers) GetUserOwnedResources(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "users"); !ok {
		return
	}
	target := strings.TrimSpace(r.PathValue("userID"))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}
	owned, err := h.ownedBy(target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read what this account owns"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"owns": owned, "count": owned.count()})
}

// Deleting the account for good.
//
// Disabling is the right default and stays the default: it keeps the audit
// trail attributable. But an account sometimes has to go - a person exercising
// their right to erasure, a test account, a mistake made at creation - and
// until now there was no way to remove one at all. An administrator could
// disable it and then watch it hold an organization open forever, invisible on
// every screen that hides disabled users.
//
// Three rules make this safe enough to expose:
//
//   - the account must already be disabled. Deletion is the second step, never
//     the first, so nobody removes an active colleague with one click.
//   - what it owns is transferred first, exactly as a deactivation does, or the
//     request is refused with the list of what would have been orphaned.
//   - the audit event records the username, because the account it names is
//     about to stop existing and the trail has to stay readable without it.
//
// Personal secrets are deleted rather than left behind: they belong to a person
// who is being removed, and an inherited secret is somebody else's credential.
func (h Handlers) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	target := strings.TrimSpace(r.PathValue("userID"))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}
	if strings.EqualFold(target, identity.UserID()) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "you cannot delete your own account"})
		return
	}

	account, found, err := h.findUser(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read the account: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such account"})
		return
	}
	if account.Enabled {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "disable the account before deleting it: deletion removes the account, disabling stops access and keeps the record",
			"code":  "disable_first",
		})
		return
	}

	var req deactivateUserRequest
	if r.Body != nil && r.ContentLength != 0 {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a valid successor is required"})
			return
		}
	}
	successor := strings.TrimSpace(req.SuccessorUserID)

	owned, err := h.ownedBy(target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read what this account owns"})
		return
	}
	if owned.count() > 0 && successor == "" {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": fmt.Sprintf("this account still owns %d resource(s); name a successor to receive them", owned.count()),
			"code":  "successor_required",
			"owns":  owned,
		})
		return
	}
	if successor != "" && strings.EqualFold(successor, target) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the successor cannot be the account being deleted"})
		return
	}

	transferred := transferReport{}
	if successor != "" && owned.count() > 0 {
		transferred, err = h.transferOwnership(target, successor)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "transfer failed, the account was left in place: " + err.Error()})
			return
		}
	}

	revoked := h.revokeAllTokens(target)
	secretsRemoved := h.deleteAllSecrets(target)

	// Last, and only once everything above succeeded: this is the step that
	// cannot be undone.
	if err := h.keycloak.DeleteUser(target); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider refused to delete the account: " + err.Error()})
		return
	}

	h.emitAudit(r, identity.UserID(), "user.delete", "user", target, "", "success", "", map[string]any{
		"username":       account.Username,
		"email":          account.Email,
		"successor":      successor,
		"transferred":    transferred.count(),
		"tokensRevoked":  revoked,
		"secretsDeleted": secretsRemoved,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":        target,
		"username":       account.Username,
		"successor":      successor,
		"transferred":    transferred,
		"tokensRevoked":  revoked,
		"secretsDeleted": secretsRemoved,
		"note":           "the audit trail keeps the events this account produced, under its identifier",
	})
}

// findUser looks an account up by id or by username, the way every other admin
// route accepts either.
func (h Handlers) findUser(identifier string) (keycloak.User, bool, error) {
	users, err := h.keycloak.ListUsers()
	if err != nil {
		return keycloak.User{}, false, err
	}
	for _, user := range users {
		if strings.EqualFold(user.ID, identifier) || strings.EqualFold(user.Username, identifier) {
			return user, true, nil
		}
	}
	return keycloak.User{}, false, nil
}

// deleteAllSecrets removes the personal secrets of an account being deleted.
// They are not transferred: a secret belongs to a person, and handing one on
// would give a successor a credential issued to somebody else.
func (h Handlers) deleteAllSecrets(userID string) int {
	if h.secretStore == nil {
		return 0
	}
	items, err := h.secretStore.ListByUser(userID)
	if err != nil {
		return 0
	}
	removed := 0
	for _, item := range items {
		if h.secretStore.Delete(userID, item.Name) == nil {
			removed++
		}
	}
	return removed
}
