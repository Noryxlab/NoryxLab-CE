package keycloak

import (
	"bytes"
	"encoding/json"
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
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Enabled  bool   `json:"enabled"`
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

func (c *Client) ListOrganizationMembers(organizationID string) ([]User, error) {
	var users []User
	if err := c.adminJSON(http.MethodGet, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID))+"/members?max=200", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) AddOrganizationMember(organizationID, userID string) error {
	if err := c.adminJSON(http.MethodPost, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID))+"/members", strings.TrimSpace(userID), nil); err != nil {
		return err
	}
	c.invalidateMembership(userID)
	return nil
}

func (c *Client) RemoveOrganizationMember(organizationID, userID string) error {
	if err := c.adminJSON(http.MethodDelete, "organizations/"+url.PathEscape(strings.TrimSpace(organizationID))+"/members/"+url.PathEscape(strings.TrimSpace(userID)), nil, nil); err != nil {
		return err
	}
	c.invalidateMembership(userID)
	return nil
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

	userID := identifier
	if !looksLikeUUID(userID) {
		users, err := c.ListUsers()
		if err != nil {
			return false, err
		}
		userID = ""
		for _, user := range users {
			if strings.EqualFold(user.Username, identifier) || strings.EqualFold(user.Email, identifier) {
				userID = user.ID
				break
			}
		}
	}
	if userID == "" {
		return false, nil
	}
	var organizations []Organization
	if err := c.adminJSON(http.MethodGet, "organizations/members/"+url.PathEscape(userID)+"/organizations", nil, &organizations); err != nil {
		return false, err
	}
	hasOrganization := len(organizations) > 0
	c.membershipMu.Lock()
	c.memberships[identifier] = cachedMembership{hasOrganization: hasOrganization, expiresAt: time.Now().Add(30 * time.Second)}
	c.memberships[userID] = cachedMembership{hasOrganization: hasOrganization, expiresAt: time.Now().Add(30 * time.Second)}
	c.membershipMu.Unlock()
	return hasOrganization, nil
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
		return fmt.Errorf("keycloak admin api %s status=%d body=%s", path, resp.StatusCode, string(responseBody))
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(output)
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
