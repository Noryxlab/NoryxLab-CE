package keycloak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Enabled  bool   `json:"enabled"`
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
	}, nil
}

func (c *Client) ListUsers() ([]User, error) {
	token, err := c.adminToken()
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/admin/realms/%s/users?max=200&briefRepresentation=true", c.baseURL, url.PathEscape(c.realm))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak users api status=%d body=%s", resp.StatusCode, string(body))
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
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
