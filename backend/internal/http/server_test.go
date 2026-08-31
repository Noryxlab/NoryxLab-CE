package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
)

func mux(t *testing.T) http.Handler {
	t.Helper()
	return NewServer(config.Config{}, handlers.Handlers{}).Handler
}

func status(t *testing.T, handler http.Handler, method, path string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Code, recorder.Header().Get("Content-Type")
}

// An unknown API path must answer as an API, not as the application. It used
// to return the home page with 200, which looks alive in a browser and is
// undebuggable from a client - and which hid the Enterprise routes going
// unregistered for an entire release.
func TestUnknownAPIPathIsNotTheHomePage(t *testing.T) {
	code, contentType := status(t, mux(t), http.MethodGet, "/api/v1/does-not-exist")
	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("content type = %q, want JSON", contentType)
	}
}

// The Community edition must not serve the Enterprise surfaces, and it must
// say so with a 404: the routes are absent, not forbidden. A 403 would
// advertise a feature waiting to be unlocked, which is what the environment
// variable used to do.
func TestCommunityDoesNotServeEnterpriseRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/v1/admin/backups/runs",
		"/api/v1/admin/audit",
		"/api/v1/egress/profiles",
		"/api/v1/assistant/chat",
	} {
		code, contentType := status(t, mux(t), http.MethodGet, path)
		if code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", path, code)
		}
		if !strings.HasPrefix(contentType, "application/json") {
			t.Fatalf("%s: content type = %q, want JSON", path, contentType)
		}
	}
}

// A known Community route stays reachable: the catch-all must not shadow it.
func TestKnownRoutesStillWin(t *testing.T) {
	code, _ := status(t, mux(t), http.MethodGet, "/api/v1/projects")
	if code == http.StatusNotFound {
		t.Fatal("a registered route was captured by the /api/ catch-all")
	}
}
