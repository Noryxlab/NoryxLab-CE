package handlers

import (
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

func TestAppBootstrapDefaultsToProjectEntrypoint(t *testing.T) {
	script := appBootstrapScript(9000, "", nil)
	for _, expected := range []string{
		"export PORT=9000 NORYX_APP_PORT=9000",
		"elif [ -f /mnt/app.sh ]; then",
		"exec /mnt/app.sh",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("app bootstrap does not contain %q", expected)
		}
	}
}

func TestAppIdentityMatchesSupportedIdentifiers(t *testing.T) {
	identity := auth.Identity{Subject: "kc-uuid", Username: "stef", Email: "stef@example.org"}
	for _, value := range []string{"kc-uuid", "stef", "stef@example.org"} {
		if !appIdentityMatches(identity, value) {
			t.Fatalf("identity should match %q", value)
		}
	}
	if appIdentityMatches(identity, "someone-else") {
		t.Fatal("identity must not match another user")
	}
}

func TestAppBootstrapKeepsExplicitCommandAsOverride(t *testing.T) {
	script := appBootstrapScript(8080, "python3 server.py", nil)
	if !strings.Contains(script, "python3 server.py") {
		t.Fatal("app bootstrap must preserve an explicit UI command")
	}
}
