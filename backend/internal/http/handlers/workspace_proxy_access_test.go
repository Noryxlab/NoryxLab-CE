package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// The workspace proxy is where two of this week's defects lived, and it had
// almost no tests: a workspace that launched and then refused entry, and an
// interface that authenticated without creating the session the proxy reads.
// Both left the platform answering correctly, which is exactly what a test
// exists to see through.

// upstreamWorkspace stands in for the workspace's own web server. The proxy
// dials port 8888 on the service name, so the fake has to listen there; the
// test skips rather than fails if the port is taken, because a busy port on a
// developer's machine says nothing about this code.
func upstreamWorkspace(t *testing.T, record *http.Request) (stop func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:8888")
	if err != nil {
		t.Skipf("port 8888 is not available for the fake workspace: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*record = *r.Clone(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("workspace"))
	})}
	go func() { _ = server.Serve(listener) }()
	return func() { _ = server.Close() }
}

func workspaceProxyFixture(t *testing.T, role access.Role) (Handlers, workspace.Workspace, project.Project) {
	t.Helper()
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Proxy project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	if role != "" {
		accessStore.SetRole(item.ID, "member", role)
	}
	workspaces := memory.NewWorkspaceStore()
	record := workspace.New("jupyter", item.ID, "w", "image", "pod", "127.0.0.1", "1", "1Gi", "", "jupyter-secret")
	record.Status = "running"
	if err := workspaces.Create(record); err != nil {
		t.Fatal(err)
	}
	sessions := memory.NewSessionStore()

	return Handlers{
		projectStore:       projects,
		accessStore:        accessStore,
		workspaceStore:     workspaces,
		sessionStore:       sessions,
		workspaceNamespace: "noryx-loads",
		authMode:           "oidc",
	}, record, item
}

func futureTime() time.Time { return time.Now().UTC().Add(time.Hour) }

func withSession(t *testing.T, h Handlers, userID string) *http.Cookie {
	t.Helper()
	token := "test-session-token"
	if err := h.sessionStore.Create(session.Session{Token: token, Identity: userID, ExpiresAt: futureTime()}); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessionCookie, Value: token}
}

// Without a session there is no entry - and the answer says so, rather than
// letting the request through to a workspace that would have served it.
func TestTheProxyRefusesARequestWithNoSession(t *testing.T) {
	h, record, _ := workspaceProxyFixture(t, access.RoleEditor)

	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+record.ID+"/", nil)
	request.SetPathValue("workspaceID", record.ID)
	recorder := httptest.NewRecorder()
	h.ProxyWorkspace(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a session, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// A member of the project without the right to launch must not reach a
// workspace either: reading a project is not entering a machine inside it.
func TestTheProxyRefusesAViewer(t *testing.T) {
	h, record, _ := workspaceProxyFixture(t, access.RoleViewer)
	cookie := withSession(t, h, "member")

	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+record.ID+"/", nil)
	request.SetPathValue("workspaceID", record.ID)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	h.ProxyWorkspace(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a viewer, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestTheProxyAnswers404ForAWorkspaceThatDoesNotExist(t *testing.T) {
	h, _, _ := workspaceProxyFixture(t, access.RoleEditor)
	cookie := withSession(t, h, "member")

	request := httptest.NewRequest(http.MethodGet, "/workspaces/nope/", nil)
	request.SetPathValue("workspaceID", "nope")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	h.ProxyWorkspace(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

// An editor gets in, and the workspace's own token is added on the way - on
// the internal hop only. A token that reached the browser would be a way into
// the workspace that bypasses the platform entirely.
func TestAnEditorReachesTheWorkspaceAndTheTokenStaysInternal(t *testing.T) {
	var upstream http.Request
	stop := upstreamWorkspace(t, &upstream)
	defer stop()

	h, record, _ := workspaceProxyFixture(t, access.RoleEditor)
	cookie := withSession(t, h, "member")

	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+record.ID+"/lab", nil)
	request.SetPathValue("workspaceID", record.ID)
	request.SetPathValue("path", "lab")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	h.ProxyWorkspace(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("an editor must reach the workspace, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := upstream.URL.Query().Get("token"); got != "jupyter-secret" {
		t.Errorf("the workspace token must be added on the internal hop, got %q", got)
	}
	if strings.Contains(recorder.Body.String(), "jupyter-secret") {
		t.Error("the workspace token must never reach the browser")
	}
	if upstream.Header.Get("X-Forwarded-Prefix") != "/workspaces/"+record.ID {
		t.Errorf("the workspace needs its public prefix to build its own URLs, got %q", upstream.Header.Get("X-Forwarded-Prefix"))
	}
}

// The guard against being sent somewhere else after signing in. An open
// redirect on a login flow is how a session ends up in somebody else's hands.
func TestReturnToOnlyAcceptsALocalPath(t *testing.T) {
	for raw, want := range map[string]string{
		"/projects":                       "/projects",
		"/projects/p1/workspaces":         "/projects/p1/workspaces",
		"":                                "",
		"https://elsewhere.example/steal": "",
		"//elsewhere.example/steal":       "",
		"/auth/realms/noryx":              "",
		"/api/v1/auth/login":              "",
		"javascript:alert(1)":             "",
	} {
		if got := safeLocalReturnTo(raw); got != want {
			t.Errorf("safeLocalReturnTo(%q) = %q, want %q", raw, got, want)
		}
	}
}

// This address is one a person keeps: a browser bookmarks it, and it gets
// pasted into messages. A workspace is deleted far more often than renamed, so
// the common way to arrive here is an old link - and the answer was a bare
// JSON error rendered as text in the address bar, which reads as a broken
// platform rather than a stale link.
func TestAStaleWorkspaceLinkExplainsItselfToABrowser(t *testing.T) {
	h, _, _ := workspaceProxyFixture(t, access.RoleEditor)

	request := httptest.NewRequest(http.MethodGet, "/workspaces/does-not-exist/", nil)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.AddCookie(withSession(t, h, "member"))
	request.SetPathValue("workspaceID", "does-not-exist")
	recorder := httptest.NewRecorder()

	h.ProxyWorkspace(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a deleted workspace answered %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Errorf("a browser was answered with %q", contentType)
	}
	body := recorder.Body.String()
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		t.Error("a browser was shown raw JSON in its address bar")
	}
	// The page has to say the work is safe, which is the actual worry of
	// somebody whose link stopped working.
	if !strings.Contains(body, "conserv") {
		t.Error("the page does not say the saved files are kept")
	}
}

// A script or the application itself still gets the JSON it parses.
func TestAStaleWorkspaceLinkStaysJSONForEverythingElse(t *testing.T) {
	h, _, _ := workspaceProxyFixture(t, access.RoleEditor)

	request := httptest.NewRequest(http.MethodGet, "/workspaces/does-not-exist/", nil)
	request.Header.Set("Accept", "application/json")
	request.AddCookie(withSession(t, h, "member"))
	request.SetPathValue("workspaceID", "does-not-exist")
	recorder := httptest.NewRecorder()

	h.ProxyWorkspace(recorder, request)

	if !strings.Contains(recorder.Body.String(), "workspace not found") {
		t.Errorf("a machine caller lost its error: %s", recorder.Body.String())
	}
}

// The lookup used to run before authentication, so the endpoint answered 404
// for an identifier that does not exist and 401 for one that does - telling a
// stranger which workspaces are real without ever signing in.
func TestAnUnauthenticatedCallerLearnsNothingAboutWhichWorkspacesExist(t *testing.T) {
	h, existing, _ := workspaceProxyFixture(t, access.RoleEditor)

	var answers []int
	// One identifier that exists and one that does not. With no credential,
	// the two must be indistinguishable.
	_ = existing
	for _, id := range []string{existing.ID, "does-not-exist"} {
		request := httptest.NewRequest(http.MethodGet, "/workspaces/"+id+"/", nil)
		request.SetPathValue("workspaceID", id)
		recorder := httptest.NewRecorder()
		h.ProxyWorkspace(recorder, request)
		answers = append(answers, recorder.Code)
	}

	for _, code := range answers {
		if code == http.StatusNotFound {
			t.Fatal("a caller with no credential was told a workspace does not exist")
		}
	}
}
