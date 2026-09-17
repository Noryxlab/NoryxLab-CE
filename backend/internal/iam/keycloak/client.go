package keycloak

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct {
	BaseURL       string
	Realm         string
	AdminRealm    string
	AdminUsername string
	AdminPassword string
}

type Client struct {
	baseURL       string
	realm         string
	adminRealm    string
	adminUsername string
	adminPassword string
	http          *http.Client
	membershipMu  sync.Mutex
	memberships   map[string]cachedMembership
}

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Enabled   bool   `json:"enabled"`
}

type Organization struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Alias   string `json:"alias"`
	Enabled bool   `json:"enabled"`
}

type cachedMembership struct {
	hasOrganization bool
	expiresAt       time.Time
}

func New(cfg Config) (*Client, error) {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("keycloak base url is required")
	}
	realm := strings.TrimSpace(cfg.Realm)
	if realm == "" {
		return nil, fmt.Errorf("keycloak realm is required")
	}
	adminRealm := strings.TrimSpace(cfg.AdminRealm)
	if adminRealm == "" {
		adminRealm = "master"
	}
	if strings.TrimSpace(cfg.AdminUsername) == "" || strings.TrimSpace(cfg.AdminPassword) == "" {
		return nil, fmt.Errorf("keycloak admin credentials are required")
	}

	return &Client{
		baseURL:       base,
		realm:         realm,
		adminRealm:    adminRealm,
		adminUsername: cfg.AdminUsername,
		adminPassword: cfg.AdminPassword,
		http:          &http.Client{Timeout: 15 * time.Second},
		memberships:   map[string]cachedMembership{},
	}, nil
}

func (c *Client) ListUsers() ([]User, error) {
	var users []User
	if err := c.adminJSON(http.MethodGet, "users?max=200&briefRepresentation=true", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// ListRealmRoleMembers returns everybody holding one realm role.
//
// One call for the whole role rather than one per account: asking each user
// what roles they hold is the obvious shape and it turns a screen of thirty
// people into thirty round trips to the directory.
func (c *Client) ListRealmRoleMembers(role string) ([]User, error) {
	var users []User
	path := "roles/" + url.PathEscape(strings.TrimSpace(role)) + "/users?max=200"
	if err := c.adminJSON(http.MethodGet, path, nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) ListOrganizations() ([]Organization, error) {
	var organizations []Organization
	if err := c.adminJSON(http.MethodGet, "organizations?max=200", nil, &organizations); err != nil {
		return nil, err
	}
	return organizations, nil
}

func (c *Client) CreateOrganization(name, alias string) (Organization, error) {
	payload := Organization{Name: strings.TrimSpace(name), Alias: strings.TrimSpace(alias), Enabled: true}
	if payload.Name == "" || payload.Alias == "" {
		return Organization{}, fmt.Errorf("organization name and alias are required")
	}
	if err := c.adminJSON(http.MethodPost, "organizations", payload, nil); err != nil {
		return Organization{}, err
	}
	organizations, err := c.ListOrganizations()
	if err != nil {
		return Organization{}, err
	}
	for _, organization := range organizations {
		if strings.EqualFold(organization.Alias, payload.Alias) {
			return organization, nil
		}
	}
	return Organization{}, fmt.Errorf("created organization was not returned by keycloak")
}

func (c *Client) DeleteOrganization(organizationID string) error {
	return c.adminJSON(http.MethodDelete, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID)), nil, nil)
}

// CreateUser adds an account to the realm and returns its identifier.
//
// The account is created enabled and with no credential: a password is set
// separately, so the two operations can be audited apart and so a caller that
// fails halfway leaves an account that cannot be signed into rather than one
// with a password nobody recorded.
func (c *Client) CreateUser(user User) (string, error) {
	payload := map[string]any{
		"username":      strings.TrimSpace(user.Username),
		"email":         strings.TrimSpace(user.Email),
		"firstName":     strings.TrimSpace(user.FirstName),
		"lastName":      strings.TrimSpace(user.LastName),
		"enabled":       true,
		"emailVerified": false,
	}
	if err := c.adminJSON(http.MethodPost, "users", payload, nil); err != nil {
		return "", err
	}
	// Keycloak answers 201 with a Location header rather than a body, and the
	// admin client here does not surface headers. Reading the account back is
	// simpler than threading them through, and confirms it exists.
	return c.resolveUserID(strings.TrimSpace(user.Username))
}

// SetTemporaryPassword replaces a user's credential with one they must change
// at their next sign-in.
//
// Temporary on purpose. An administrator resetting a password necessarily
// learns it, and a password only its owner knows is the only kind worth having:
// forcing the change bounds how long the administrator's copy is valid.
// DeleteUser removes the account from the realm. The caller is responsible for
// what the account owned: Keycloak knows nothing about projects or datasets,
// and will happily delete the only owner of half a platform.
func (c *Client) DeleteUser(identifier string) error {
	userID, err := c.resolveUserID(identifier)
	if err != nil {
		return err
	}
	c.invalidateMembership(identifier)
	return c.adminJSON(http.MethodDelete, "/users/"+url.PathEscape(userID), nil, nil)
}

func (c *Client) SetTemporaryPassword(userID, password string) error {
	return c.adminJSON(http.MethodPut,
		"users/"+url.PathEscape(strings.TrimSpace(userID))+"/reset-password",
		map[string]any{"type": "password", "value": password, "temporary": true}, nil)
}

// SetUserEnabled turns an account on or off. Disabling is preferred to
// deletion: it stops access immediately and keeps the audit trail attributable.
func (c *Client) SetUserEnabled(userID string, enabled bool) error {
	return c.adminJSON(http.MethodPut,
		"users/"+url.PathEscape(strings.TrimSpace(userID)),
		map[string]any{"enabled": enabled}, nil)
}

func (c *Client) ListOrganizationMembers(organizationID string) ([]User, error) {
	var users []User
	if err := c.adminJSON(http.MethodGet, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID))+"/members?max=200", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// Organization membership takes a user id, and callers hold a username.
//
// Keycloak identifies a member by its own identifier, and every screen and
// script that reaches for one holds a username instead - so the call went out
// with a name where an id belonged and came back 404. Resolving here rather
// than at each caller: the identifier a caller happens to hold is not a
// property the membership API should be sensitive to, and DeleteUser has
// resolved it this way since it was written.
//
// A name nobody can resolve is an explicit error rather than an empty string
// sent onward. Sent onward, it becomes a 404 from Keycloak that reads as "no
// such organization", which points at the wrong object entirely.
func (c *Client) AddOrganizationMember(organizationID, userID string) error {
	resolved, err := c.requireUserID(userID)
	if err != nil {
		return err
	}
	if err := c.adminJSON(http.MethodPost, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID))+"/members", resolved, nil); err != nil {
		return err
	}
	c.invalidateMembership(userID)
	c.invalidateMembership(resolved)
	return nil
}

func (c *Client) RemoveOrganizationMember(organizationID, userID string) error {
	resolved, err := c.requireUserID(userID)
	if err != nil {
		return err
	}
	if err := c.adminJSON(http.MethodDelete, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID))+"/members/"+url.PathEscape(resolved), nil, nil); err != nil {
		return err
	}
	// Both spellings: the cache is keyed by whatever the caller used, and a
	// membership that stays cached after a removal is a right that survives
	// being taken away.
	c.invalidateMembership(userID)
	c.invalidateMembership(resolved)
	return nil
}

// requireUserID resolves an identifier and refuses to continue without one.
func (c *Client) requireUserID(identifier string) (string, error) {
	resolved, err := c.resolveUserID(identifier)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved) == "" {
		return "", ErrNoSuchUser
	}
	return resolved, nil
}

func (c *Client) HasOrganization(identifier string) (bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return false, nil
	}
	c.membershipMu.Lock()
	cached, ok := c.memberships[identifier]
	c.membershipMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.hasOrganization, nil
	}

	userID, err := c.resolveUserID(identifier)
	if err != nil {
		return false, err
	}
	if userID == "" {
		return false, nil
	}
	organizations, err := c.ListUserOrganizations(userID)
	if err != nil {
		return false, err
	}
	hasOrganization := len(organizations) > 0
	c.membershipMu.Lock()
	c.memberships[identifier] = cachedMembership{hasOrganization: hasOrganization, expiresAt: time.Now().Add(30 * time.Second)}
	c.memberships[userID] = cachedMembership{hasOrganization: hasOrganization, expiresAt: time.Now().Add(30 * time.Second)}
	c.membershipMu.Unlock()
	return hasOrganization, nil
}

func (c *Client) ListUserOrganizations(identifier string) ([]Organization, error) {
	userID, err := c.resolveUserID(identifier)
	if err != nil {
		return nil, err
	}
	if userID == "" {
		return []Organization{}, nil
	}
	var organizations []Organization
	if err := c.adminJSON(http.MethodGet, "organizations/members/"+url.PathEscape(userID)+"/organizations", nil, &organizations); err != nil {
		return nil, err
	}
	return organizations, nil
}

// ClientAudiences returns the audiences a client's mappers add to its tokens.
//
// The platform requires an audience and the realm has to be configured to
// issue it. Nothing connected the two, so a realm missing that mapper produced
// a platform that authenticated a user and then refused every request they
// made, with no way to see why from either side.
func (c *Client) ClientAudiences(clientID string) ([]string, error) {
	var clients []struct {
		ID string `json:"id"`
	}
	if err := c.adminJSON(http.MethodGet,
		"clients?clientId="+url.QueryEscape(strings.TrimSpace(clientID)), nil, &clients); err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("no client %q in realm %s", clientID, c.realm)
	}

	var mappers []struct {
		ProtocolMapper string            `json:"protocolMapper"`
		Config         map[string]string `json:"config"`
	}
	if err := c.adminJSON(http.MethodGet,
		"clients/"+url.PathEscape(clients[0].ID)+"/protocol-mappers/models", nil, &mappers); err != nil {
		return nil, err
	}

	audiences := []string{}
	for _, mapper := range mappers {
		if mapper.ProtocolMapper != "oidc-audience-mapper" {
			continue
		}
		// Keycloak stores the audience under one of two keys depending on
		// whether it names a client or a free-form value.
		for _, key := range []string{"included.client.audience", "included.custom.audience"} {
			if value := strings.TrimSpace(mapper.Config[key]); value != "" {
				audiences = append(audiences, value)
			}
		}
	}
	return audiences, nil
}

func (c *Client) resolveUserID(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || looksLikeUUID(identifier) {
		return identifier, nil
	}
	users, err := c.ListUsers()
	if err != nil {
		return "", err
	}
	for _, user := range users {
		if strings.EqualFold(user.Username, identifier) || strings.EqualFold(user.Email, identifier) {
			return user.ID, nil
		}
	}
	return "", nil
}

func (c *Client) invalidateMembership(identifier string) {
	c.membershipMu.Lock()
	delete(c.memberships, strings.TrimSpace(identifier))
	c.membershipMu.Unlock()
}

func looksLikeUUID(value string) bool {
	return len(value) == 36 && strings.Count(value, "-") == 4
}

func (c *Client) adminJSON(method, path string, payload any, output any) error {
	token, err := c.adminToken()
	if err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	endpoint := fmt.Sprintf("%s/admin/realms/%s/%s", c.baseURL, url.PathEscape(c.realm), strings.TrimPrefix(path, "/"))
	// An empty path addresses the realm itself. Left as it is the URL ends in a
	// slash, which Keycloak answers with 404 - so the realm object would be
	// unreachable through this helper for no reason anybody could see.
	endpoint = strings.TrimSuffix(endpoint, "/")
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		return &APIError{Path: path, StatusCode: resp.StatusCode, Body: string(responseBody)}
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(output)
}

// APIError carries the status Keycloak answered with, so a caller can tell
// "no such organization" from "the identity provider is unreachable".
//
// It used to be a formatted string. Every failure therefore became a 502, and
// an operator who mistyped an identifier went looking for a Keycloak outage.
type APIError struct {
	Path       string
	StatusCode int
	// Body is Keycloak's own response. Useful in a log and never in an API
	// response: it is another system's internals, and the caller cannot act on
	// it.
	Body string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("keycloak admin api %s status=%d body=%s", e.Path, e.StatusCode, e.Body)
}

// IsConflict reports whether Keycloak refused because the object already
// exists - a username or email already taken, most often.
func IsConflict(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}

// ErrNoSuchUser is a name the directory does not know.
//
// Distinct from a 404 out of Keycloak on purpose: those are indistinguishable
// from each other once they cross a handler, and a member operation that
// reported "no such organization" sent an administrator looking at the
// organization listed in front of them.
var ErrNoSuchUser = errors.New("no such user")

// IsNoSuchUser reports whether the identifier named nobody.
func IsNoSuchUser(err error) bool { return errors.Is(err, ErrNoSuchUser) }

// IsNotFound reports whether Keycloak answered 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func (c *Client) adminToken() (string, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "admin-cli")
	form.Set("username", c.adminUsername)
	form.Set("password", c.adminPassword)

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, url.PathEscape(c.adminRealm))
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("keycloak token api status=%d body=%s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("missing access_token in keycloak token response")
	}
	return tokenResp.AccessToken, nil
}
