package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testIssuer = "https://example.test/auth/realms/noryx"

// signingFixture serves a JWKS with one key and signs tokens with it.
func signingFixture(t *testing.T) (*OIDCVerifier, func(jwt.MapClaims) string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key"
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	t.Cleanup(server.Close)

	verifier, err := NewOIDCVerifier(testIssuer, server.URL, "noryx-api")
	if err != nil {
		t.Fatal(err)
	}

	sign := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = kid
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	return verifier, sign
}

// Each refusal must name its own cause.
//
// Six of them reach the caller as one opaque "invalid bearer token", which is
// right - a caller learns nothing from a detailed refusal. But the operator has
// to tell them apart, and the error is what carries that. Without it,
// diagnosing a failed login is guesswork against a platform that knows the
// answer and will not say it.
func TestRefusalsExplainThemselvesToTheOperator(t *testing.T) {
	verifier, sign := signingFixture(t)
	now := time.Now()

	cases := map[string]struct {
		token string
		says  string
	}{
		"not a jwt": {"clearly-not-a-token", ""},
		"unknown key id": {
			"eyJhbGciOiJSUzI1NiIsImtpZCI6InVua25vd24ifQ.eyJzdWIiOiJhIn0.c2ln",
			"not found in the JWKS",
		},
		// The commonest misconfiguration in the field, and the one least
		// guessable from outside: a perfectly valid token that simply does not
		// carry the audience this platform requires.
		"wrong audience": {
			sign(jwt.MapClaims{
				"iss": testIssuer, "sub": "alice", "aud": "somebody-else",
				"exp": now.Add(time.Hour).Unix(), "iat": now.Unix(),
			}),
			"audience",
		},
		"wrong issuer": {
			sign(jwt.MapClaims{
				"iss": "https://elsewhere.test/auth/realms/noryx", "sub": "alice",
				"aud": "noryx-api", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(),
			}),
			"",
		},
		"expired": {
			sign(jwt.MapClaims{
				"iss": testIssuer, "sub": "alice", "aud": "noryx-api",
				"exp": now.Add(-time.Hour).Unix(), "iat": now.Add(-2 * time.Hour).Unix(),
			}),
			"expired",
		},
	}

	for name, c := range cases {
		_, err := verifier.VerifyBearerToken(c.token)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s: refused without saying why", name)
		}
		if c.says != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.says)) {
			t.Errorf("%s: error %q does not mention %q", name, err, c.says)
		}
	}
}

func TestAValidTokenIsAccepted(t *testing.T) {
	verifier, sign := signingFixture(t)
	now := time.Now()
	token := sign(jwt.MapClaims{
		"iss": testIssuer, "sub": "alice-id", "aud": "noryx-api",
		"preferred_username": "alice", "email": "alice@example.test",
		"exp": now.Add(time.Hour).Unix(), "iat": now.Unix(),
	})

	identity, err := verifier.VerifyBearerToken(token)
	if err != nil {
		t.Fatalf("a well-formed token was refused: %v", err)
	}
	if identity.Username != "alice" || identity.Email != "alice@example.test" {
		t.Fatalf("identity = %+v", identity)
	}
}
