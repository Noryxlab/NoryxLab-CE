package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// A principal that is not a person.
//
// An app belongs to FOR and goes into production; the person who launched it
// leaves; who at FOR answers for it? Today the platform's answer is to name a
// successor at deactivation and hand everything over - which works, and moves
// the responsibility from one human to the next. Each hop is a decision
// somebody has to make on the worst possible day, and nothing in the platform
// can hold a production responsibility that does not resign.
//
// A service account can. It is an account in the same directory, in an
// organization, able to hold project roles and to own what must outlive the
// people who set it up. What it cannot do is sign in: no password, no reset,
// no browser. A credential it holds is a token, issued by an administrator on
// its behalf, and revocable one at a time - a credential is something the
// account has, never the account itself. That distinction is the whole of
// ADR-039.
//
// It acts as itself. A token belonging to it produces its identity, not the
// platform's, so every authorization path already written works unchanged and
// the audit reads `for-production` in the same column it reads a person's
// name. The one thing the screens owe a reader is to say which actors are not
// people - an audit that treats them alike is honest, an interface that hides
// the difference is not.

const (
	// serviceAccountRole marks the accounts that are not people. A realm role
	// rather than an attribute, because the platform already marks its
	// administrators this way and a role is listed in one call instead of one
	// per account.
	serviceAccountRole = "noryx-service-account"
	// responsibleAttribute is who answers for this account. Not optional: a
	// non-human account that can own regulated data and answers to nobody is
	// the most convenient way to hold an HDS dataset no one is accountable for.
	responsibleAttribute = "noryx-responsible"
	purposeAttribute     = "noryx-purpose"
)

type serviceAccountRequest struct {
	Username       string `json:"username"`
	Purpose        string `json:"purpose"`
	OrganizationID string `json:"organizationId"`
	// ResponsibleUserID is the person who answers for what this account does.
	ResponsibleUserID string `json:"responsibleUserId"`
}

type serviceAccountView struct {
	Username      string   `json:"username"`
	Purpose       string   `json:"purpose,omitempty"`
	Responsible   string   `json:"responsible,omitempty"`
	Organizations []string `json:"organizations,omitempty"`
	Enabled       bool     `json:"enabled"`
	// Tokens is how many credentials are live, because an account with none
	// does nothing and an account with five is a question.
	Tokens int `json:"tokens"`
}

func (h Handlers) serviceAccountUsernames() (map[string]bool, error) {
	names := map[string]bool{}
	if h.keycloak == nil {
		return names, nil
	}
	members, err := h.keycloak.ListRealmRoleMembers(serviceAccountRole)
	if errors.Is(err, keycloak.ErrRoleNotFound) {
		// No service account has ever been marked on this installation, so the
		// role does not exist. That is zero accounts, not a failure: treating
		// it as one made every user listing log a 404 and drop the marker that
		// says which actors are not people.
		return names, nil
	}
	if err != nil {
		return names, err
	}
	for _, member := range members {
		if name := strings.ToLower(strings.TrimSpace(member.Username)); name != "" {
			names[name] = true
		}
	}
	return names, nil
}

func (h Handlers) ListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "users"); !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	members, err := h.keycloak.ListRealmRoleMembers(serviceAccountRole)
	if err != nil {
		// An installation that has never created one has no role either, and
		// that is an empty list rather than a failure.
		writeJSON(w, http.StatusOK, map[string]any{"items": []serviceAccountView{}})
		return
	}
	live := h.liveTokenCounts()
	items := make([]serviceAccountView, 0, len(members))
	for _, member := range members {
		view := serviceAccountView{
			Username:      member.Username,
			Enabled:       member.Enabled,
			Organizations: h.organizationNamesFor(member),
			Tokens:        live[strings.ToLower(strings.TrimSpace(member.Username))],
		}
		// Attributes are not in a listing, so they are read per account. There
		// are few service accounts by construction; if that stops being true,
		// this is the loop to change.
		if full, found, err := h.keycloak.GetUser(member.Username); err == nil && found {
			view.Responsible = firstAttribute(full, responsibleAttribute)
			view.Purpose = firstAttribute(full, purposeAttribute)
		}
		items = append(items, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	var req serviceAccountRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a name is required"})
		return
	}
	responsible := strings.TrimSpace(req.ResponsibleUserID)
	if responsible == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "name the person who answers for this account",
			"code":  "responsible_required",
		})
		return
	}
	if _, found, err := h.keycloak.GetUser(responsible); err != nil || !found {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no account named " + responsible})
		return
	}
	if h.organizationRequired && strings.TrimSpace(req.OrganizationID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "this installation requires organization membership; choose one",
			"code":  "organization_required",
		})
		return
	}

	if err := h.keycloak.EnsureRealmRole(serviceAccountRole,
		"Marks an account that is not a person: no interactive sign-in, credentials are API tokens"); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not prepare the service account role: " + err.Error()})
		return
	}
	// No password, ever. An account that can sign in interactively is a shared
	// password with extra steps, which is the thing this replaces.
	userID, err := h.keycloak.CreateUser(keycloak.User{
		Username: username,
		Attributes: map[string][]string{
			responsibleAttribute: {responsible},
			purposeAttribute:     {strings.TrimSpace(req.Purpose)},
		},
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not create the account: " + err.Error()})
		return
	}
	if err := h.keycloak.AddRealmRole(userID, serviceAccountRole); err != nil {
		// Unmarked, it would appear in the people screen as a person nobody
		// can explain, so this is a failure rather than a warning.
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":  "the account was created but could not be marked as a service account",
			"userId": userID,
		})
		return
	}
	if organizationID := strings.TrimSpace(req.OrganizationID); organizationID != "" {
		if err := h.keycloak.AddOrganizationMember(organizationID, userID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":  "the account was created but could not be added to the organization",
				"userId": userID,
			})
			return
		}
	}
	h.emitAudit(r, identity.UserID(), "service-account.created", "service-account", username, "", "success", "",
		map[string]any{"responsible": responsible, "organizationId": strings.TrimSpace(req.OrganizationID)})
	writeJSON(w, http.StatusCreated, serviceAccountView{
		Username: username, Purpose: strings.TrimSpace(req.Purpose),
		Responsible: responsible, Enabled: true,
	})
}

// CreateServiceAccountToken issues a credential that acts as the account.
//
// Issued by an administrator because the account has no browser to issue one
// from, and owned by the account rather than by the administrator: the token
// carries the account's name, so revoking the person who created it changes
// nothing, which is the point of the whole feature.
func (h Handlers) CreateServiceAccountToken(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.apiTokenStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api tokens are not configured"})
		return
	}
	username := strings.ToLower(strings.TrimSpace(r.PathValue("username")))
	accounts, err := h.serviceAccountUsernames()
	if err != nil || !accounts[username] {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no service account named " + username})
		return
	}

	var req struct {
		Name          string   `json:"name"`
		Scopes        []string `json:"scopes"`
		ExpiresInDays int      `json:"expiresInDays"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	for _, scope := range req.Scopes {
		if !apitoken.ValidScope(scope) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown scope " + strings.TrimSpace(scope)})
			return
		}
	}
	id, secret, err := newTokenParts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate a token"})
		return
	}
	sum := sha256.Sum256([]byte(secret))
	token := apitoken.Token{
		ID: id, UserID: username, Name: firstNonEmpty(strings.TrimSpace(req.Name), username),
		Scopes:    apitoken.NormalizeScopes(req.Scopes),
		CreatedAt: time.Now().UTC(), SecretHash: sum[:],
	}
	if req.ExpiresInDays > 0 {
		expiry := token.CreatedAt.AddDate(0, 0, req.ExpiresInDays)
		token.ExpiresAt = &expiry
	}
	if err := h.apiTokenStore.Put(token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the token"})
		return
	}
	h.emitAudit(r, identity.UserID(), "service-account.token.created", "service-account", username, "", "success", "",
		map[string]any{"tokenId": id, "scopes": token.Scopes})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  token,
		"secret": tokenPrefix + "_" + id + "_" + secret,
		"note":   "this secret is shown once and cannot be recovered",
	})
}

// DisableServiceAccount stops it acting, and says what stopped.
//
// Disabled rather than deleted, like a person: deleting the actor breaks the
// audit trail the moment somebody asks what it did. Its tokens go with it -
// an account disabled while its credentials still work is not disabled.
func (h Handlers) DisableServiceAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	username := strings.ToLower(strings.TrimSpace(r.PathValue("username")))
	accounts, err := h.serviceAccountUsernames()
	if err != nil || !accounts[username] {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no service account named " + username})
		return
	}
	revoked := 0
	if h.apiTokenStore != nil {
		if all, err := h.apiTokenStore.ListAll(); err == nil {
			now := time.Now().UTC()
			for _, token := range all {
				if !strings.EqualFold(token.UserID, username) || token.RevokedAt != nil {
					continue
				}
				if _, err := h.apiTokenStore.Revoke(token.ID, token.UserID, now); err == nil {
					revoked++
				}
			}
		}
	}
	if err := h.keycloak.SetUserEnabled(username, false); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not disable the account: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "service-account.disabled", "service-account", username, "", "success", "",
		map[string]any{"tokensRevoked": revoked})
	writeJSON(w, http.StatusOK, map[string]any{"username": username, "tokensRevoked": revoked})
}

func (h Handlers) liveTokenCounts() map[string]int {
	counts := map[string]int{}
	if h.apiTokenStore == nil {
		return counts
	}
	all, err := h.apiTokenStore.ListAll()
	if err != nil {
		return counts
	}
	now := time.Now().UTC()
	for _, token := range all {
		if token.Active(now) {
			counts[strings.ToLower(strings.TrimSpace(token.UserID))]++
		}
	}
	return counts
}

func (h Handlers) organizationNamesFor(user keycloak.User) []string {
	if h.keycloak == nil {
		return nil
	}
	identifier := firstNonEmpty(user.Username, user.Email)
	organizations, err := h.keycloak.ListUserOrganizations(identifier)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(organizations))
	for _, organization := range organizations {
		names = append(names, firstNonEmpty(organization.Name, organization.Alias))
	}
	return names
}

func firstAttribute(user keycloak.User, key string) string {
	values := user.Attributes[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
