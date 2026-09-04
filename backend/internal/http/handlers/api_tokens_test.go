package handlers

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func tokenFixture(t *testing.T, userID string, mutate func(*apitoken.Token)) (Handlers, string) {
	t.Helper()
	store := memory.NewAPITokenStore()
	handlers := Handlers{apiTokenStore: store, authMode: "oidc"}

	id, secret, err := newTokenParts()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(secret))
	token := apitoken.Token{
		ID: id, UserID: userID, Name: "ci", CreatedAt: time.Now().UTC(), SecretHash: sum[:],
	}
	if mutate != nil {
		mutate(&token)
	}
	if err := store.Put(token); err != nil {
		t.Fatal(err)
	}
	return handlers, tokenPrefix + "_" + id + "_" + secret
}

func TestAPersonalTokenAuthenticatesAsItsOwner(t *testing.T) {
	handlers, secret := tokenFixture(t, "alice", nil)
	identity, ok := handlers.identityFromAPIToken(secret)
	if !ok {
		t.Fatal("a valid token was refused")
	}
	// The token acts as the person and carries no rights of its own, so the
	// blast radius of a leak is one account rather than the platform.
	if identity.UserID() != "alice" {
		t.Fatalf("identity = %q, want alice", identity.UserID())
	}
	if len(identity.Roles) != 0 {
		t.Fatalf("a personal token granted roles of its own: %v", identity.Roles)
	}
}

func TestTheSecretIsNotRecoverableFromTheStore(t *testing.T) {
	handlers, secret := tokenFixture(t, "alice", nil)
	tokens, _ := handlers.apiTokenStore.ListByUser("alice")
	for _, token := range tokens {
		if strings.Contains(string(token.SecretHash), strings.Split(secret, "_")[2]) {
			t.Fatal("the secret is recoverable from the stored token")
		}
	}
}

func TestRevokedAndExpiredTokensAreRefused(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour)

	handlers, expired := tokenFixture(t, "alice", func(token *apitoken.Token) {
		token.ExpiresAt = &past
	})
	if _, ok := handlers.identityFromAPIToken(expired); ok {
		t.Fatal("an expired token authenticated")
	}

	handlers, revoked := tokenFixture(t, "alice", func(token *apitoken.Token) {
		token.RevokedAt = &past
	})
	if _, ok := handlers.identityFromAPIToken(revoked); ok {
		t.Fatal("a revoked token authenticated")
	}
}

func TestAWrongSecretIsRefused(t *testing.T) {
	handlers, secret := tokenFixture(t, "alice", nil)
	parts := strings.Split(secret, "_")
	tampered := parts[0] + "_" + parts[1] + "_" + strings.Repeat("a", len(parts[2]))
	if _, ok := handlers.identityFromAPIToken(tampered); ok {
		t.Fatal("a tampered secret authenticated")
	}
}

func TestMalformedTokensAreRejectedBeforeAnyLookup(t *testing.T) {
	handlers, _ := tokenFixture(t, "alice", nil)
	for _, candidate := range []string{"", "noryx", "noryx_short_short", "sk-openai-style", "noryx_a_b_c"} {
		if _, ok := handlers.identityFromAPIToken(candidate); ok {
			t.Fatalf("token %q authenticated", candidate)
		}
	}
}

// Revoking is scoped to the owner: a caller must not be able to revoke
// somebody else's token by guessing an identifier, and the answer must not
// confirm that the identifier exists.
func TestOneUserCannotRevokeAnothersToken(t *testing.T) {
	store := memory.NewAPITokenStore()
	_ = store.Put(apitoken.Token{ID: "t1", UserID: "alice", Name: "ci", CreatedAt: time.Now().UTC()})

	revoked, err := store.Revoke("t1", "bob", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("bob revoked alice's token")
	}

	tokens, _ := store.ListByUser("alice")
	if len(tokens) != 1 || tokens[0].RevokedAt != nil {
		t.Fatalf("alice's token was affected: %+v", tokens)
	}
}

func TestCreatingATokenRequiresAName(t *testing.T) {
	handlers := Handlers{apiTokenStore: memory.NewAPITokenStore(), authMode: "header"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/user/api-tokens",
		strings.NewReader(`{"name":"   "}`))
	request.Header.Set(userHeader, "alice")
	recorder := httptest.NewRecorder()

	handlers.CreateAPIToken(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: an unnamed token cannot be revoked with confidence", recorder.Code)
	}
}
