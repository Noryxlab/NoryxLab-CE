package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type OIDCVerifier struct {
	issuer   string
	audience string
	jwksURL  string
	http     *http.Client
	cacheTTL time.Duration

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	lastFetch time.Time
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewOIDCVerifier(issuer, jwksURL, audience string) (*OIDCVerifier, error) {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return nil, fmt.Errorf("oidc issuer is required")
	}

	jwksURL = strings.TrimSpace(jwksURL)
	if jwksURL == "" {
		jwksURL = strings.TrimSuffix(issuer, "/") + "/protocol/openid-connect/certs"
	}

	v := &OIDCVerifier{
		issuer:   issuer,
		audience: strings.TrimSpace(audience),
		jwksURL:  jwksURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		cacheTTL: 10 * time.Minute,
		keys:     map[string]*rsa.PublicKey{},
	}

	if err := v.refreshKeys(context.Background()); err != nil {
		return nil, err
	}

	return v, nil
}

func (v *OIDCVerifier) VerifyBearerToken(token string) (Identity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Identity{}, errors.New("empty bearer token")
	}

	kid, err := readTokenKID(token)
	if err != nil {
		return Identity{}, err
	}

	if err := v.ensureKey(kid); err != nil {
		return Identity{}, err
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unsupported alg %s", t.Method.Alg())
		}
		k, ok := v.getKey(kid)
		if !ok {
			return nil, fmt.Errorf("signing key not found for kid %q", kid)
		}
		return k, nil
	}, jwt.WithIssuer(v.issuer), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return Identity{}, err
	}
	if !parsed.Valid {
		return Identity{}, errors.New("invalid token")
	}

	if v.audience != "" {
		if !claimsHasAudience(claims, v.audience) {
			return Identity{}, fmt.Errorf("invalid token audience")
		}
	}

	identity := Identity{Roles: map[string]struct{}{}}
	if sub, _ := claims["sub"].(string); sub != "" {
		identity.Subject = sub
	}
	if username, _ := claims["preferred_username"].(string); username != "" {
		identity.Username = username
	}
	if email, _ := claims["email"].(string); email != "" {
		identity.Email = email
	}

	if realmAccess, ok := claims["realm_access"].(map[string]any); ok {
		if roles, ok := realmAccess["roles"].([]any); ok {
			for _, roleAny := range roles {
				if role, ok := roleAny.(string); ok && role != "" {
					identity.Roles[role] = struct{}{}
				}
			}
		}
	}

	if identity.UserID() == "" {
		return Identity{}, errors.New("token does not contain identifiable user")
	}

	return identity, nil
}

func (v *OIDCVerifier) ensureKey(kid string) error {
	if kid == "" {
		return fmt.Errorf("token kid is missing")
	}

	if k, ok := v.getKey(kid); ok && k != nil {
		v.mu.RLock()
		stale := time.Since(v.lastFetch) > v.cacheTTL
		v.mu.RUnlock()
		if !stale {
			return nil
		}
	}

	if err := v.refreshKeys(context.Background()); err != nil {
		return err
	}

	if _, ok := v.getKey(kid); !ok {
		return fmt.Errorf("key id %q not found in JWKS", kid)
	}

	return nil
}

func (v *OIDCVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	next := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		if key.Kid == "" || key.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSAPublicKey(key)
		if err != nil {
			continue
		}
		next[key.Kid] = pub
	}
	if len(next) == 0 {
		return fmt.Errorf("no valid RSA keys in JWKS")
	}

	v.mu.Lock()
	v.keys = next
	v.lastFetch = time.Now()
	v.mu.Unlock()

	return nil
}

func (v *OIDCVerifier) getKey(kid string) (*rsa.PublicKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	k, ok := v.keys[kid]
	return k, ok
}

func readTokenKID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid jwt format")
	}

	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode jwt header: %w", err)
	}

	var header map[string]any
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return "", fmt.Errorf("unmarshal jwt header: %w", err)
	}

	kid, _ := header["kid"].(string)
	return kid, nil
}

func jwkToRSAPublicKey(jwk jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

func claimsHasAudience(claims jwt.MapClaims, audience string) bool {
	raw, ok := claims["aud"]
	if ok {
		switch aud := raw.(type) {
		case string:
			if aud == audience {
				return true
			}
		case []any:
			for _, item := range aud {
				if s, ok := item.(string); ok && s == audience {
					return true
				}
			}
		case []string:
			for _, item := range aud {
				if item == audience {
					return true
				}
			}
		}
	}

	if azp, ok := claims["azp"].(string); ok && azp == audience {
		return true
	}

	return false
}
