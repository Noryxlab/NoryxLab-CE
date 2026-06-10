package handlers

import (
	"strings"
	"testing"
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

func TestAppBootstrapKeepsExplicitCommandAsOverride(t *testing.T) {
	script := appBootstrapScript(8080, "python3 server.py", nil)
	if !strings.Contains(script, "python3 server.py") {
		t.Fatal("app bootstrap must preserve an explicit UI command")
	}
}
