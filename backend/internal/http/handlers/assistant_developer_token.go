package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// developerAssistantClaims are intentionally narrow: the token identifies one
// workspace and expires quickly. It is not an OpenAI credential.
type developerAssistantClaims struct {
	UserID      string `json:"u"`
	ProjectID   string `json:"p"`
	WorkspaceID string `json:"w"`
	ExpiresAt   int64  `json:"e"`
}

func signDeveloperAssistantToken(key string, claims developerAssistantClaims) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errors.New("assistant signing key is unavailable")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyDeveloperAssistantToken(key, token string) (developerAssistantClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if strings.TrimSpace(key) == "" || len(parts) != 2 {
		return developerAssistantClaims{}, errors.New("invalid assistant token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return developerAssistantClaims{}, errors.New("invalid assistant token")
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return developerAssistantClaims{}, errors.New("invalid assistant token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return developerAssistantClaims{}, errors.New("invalid assistant token")
	}
	var claims developerAssistantClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.UserID == "" || claims.ProjectID == "" || claims.WorkspaceID == "" || time.Now().UTC().Unix() >= claims.ExpiresAt {
		return developerAssistantClaims{}, errors.New("expired assistant token")
	}
	return claims, nil
}
