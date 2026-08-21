package handlers

import (
	"testing"
	"time"
)

func TestDeveloperAssistantTokenRoundTrip(t *testing.T) {
	key := "test-signing-key"
	claims := developerAssistantClaims{
		UserID:      "user-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		ExpiresAt:   time.Now().UTC().Add(time.Hour).Unix(),
	}
	token, err := signDeveloperAssistantToken(key, claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyDeveloperAssistantToken(key, token)
	if err != nil {
		t.Fatal(err)
	}
	if verified != claims {
		t.Fatalf("claims mismatch: %#v", verified)
	}
	if _, err := verifyDeveloperAssistantToken("other-key", token); err == nil {
		t.Fatal("token must not verify with another signing key")
	}
}

func TestDeveloperAssistantTokenExpires(t *testing.T) {
	token, err := signDeveloperAssistantToken("test-signing-key", developerAssistantClaims{
		UserID:      "user-1",
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		ExpiresAt:   time.Now().UTC().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyDeveloperAssistantToken("test-signing-key", token); err == nil {
		t.Fatal("expired token must not verify")
	}
}

func TestOpenAICompatiblePayloadRequiresMessages(t *testing.T) {
	if !openAICompatiblePayloadHasMessages(map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hello"}}}) {
		t.Fatal("payload with messages must be accepted")
	}
	for _, payload := range []map[string]any{
		{},
		{"messages": []any{}},
		{"messages": "not-a-list"},
	} {
		if openAICompatiblePayloadHasMessages(payload) {
			t.Fatalf("payload must be rejected: %#v", payload)
		}
	}
}
