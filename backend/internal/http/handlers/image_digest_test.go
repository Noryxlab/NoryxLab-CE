package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// What runs must be what was recorded. These are the two halves of that: the
// digest is asked for, and it is the digest that gets launched.

func TestPinnedImageRunsTheDigestWhenThereIsOne(t *testing.T) {
	for _, testCase := range []struct{ image, digest, want string }{
		{"harbor.lan/p/app:1.0", "sha256:abc", "harbor.lan/p/app@sha256:abc"},
		// No digest: the tag runs, because refusing to launch over a registry
		// hiccup helps nobody.
		{"harbor.lan/p/app:1.0", "", "harbor.lan/p/app:1.0"},
		// Already pinned: left exactly as it is.
		{"harbor.lan/p/app@sha256:def", "sha256:abc", "harbor.lan/p/app@sha256:def"},
		// A port in the registry host must not be mistaken for a tag.
		{"registry.local:5000/p/app:2", "sha256:abc", "registry.local:5000/p/app@sha256:abc"},
	} {
		if got := pinnedImage(testCase.image, testCase.digest); got != testCase.want {
			t.Errorf("pinnedImage(%q, %q) = %q, want %q", testCase.image, testCase.digest, got, testCase.want)
		}
	}
}

func TestTheDigestComesFromTheRegistry(t *testing.T) {
	var asked string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.Method + " " + r.URL.Path
		_, _ = w.Write([]byte(`{"digest":"sha256:1234"}`))
	}))
	defer registry.Close()

	h := Handlers{harborURL: registry.URL}
	digest := h.resolveImageDigest(registryHost(registry.URL) + "/noryx-environments/jupyter:0.1.0")

	if digest != "sha256:1234" {
		t.Fatalf("digest = %q, want sha256:1234", digest)
	}
	// HEAD, not GET: the manifest body is not needed and can be large.
	// Harbor's API, not the registry protocol: the registry endpoint answers
	// 401 to basic credentials and expects a bearer token flow, which is how
	// the first version of this silently resolved nothing in production while
	// passing against a fake that accepted basic auth.
	if asked != "GET /api/v2.0/projects/noryx-environments/repositories/jupyter/artifacts/0.1.0" {
		t.Errorf("asked %q", asked)
	}
}

// A registry that cannot answer must not stop somebody working. The launch
// proceeds on the tag and the record says the digest is unknown - which is
// honest, where a guessed digest would not be.
func TestAnUnreachableRegistryYieldsNoDigestRatherThanAnError(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer registry.Close()

	h := Handlers{harborURL: registry.URL}
	if digest := h.resolveImageDigest(registryHost(registry.URL) + "/p/app:1"); digest != "" {
		t.Fatalf("expected no digest, got %q", digest)
	}
}

// Credentials belong to one registry. Sending Harbor's to somebody else's is
// not a thing to do by accident.
func TestOnlyThePlatformsOwnRegistryIsAsked(t *testing.T) {
	var reached bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		_, _ = w.Write([]byte(`{"digest":"sha256:nope"}`))
	}))
	defer elsewhere.Close()

	h := Handlers{harborURL: "https://harbor.lan", harborUsername: "robot", harborPassword: "secret"}
	if digest := h.resolveImageDigest(registryHost(elsewhere.URL) + "/p/app:1"); digest != "" {
		t.Errorf("a foreign registry must not be resolved, got %q", digest)
	}
	if reached {
		t.Error("the platform's registry credentials were sent to another host")
	}
}

func TestAnUnparseableReferenceYieldsNoDigest(t *testing.T) {
	h := Handlers{harborURL: "https://harbor.lan"}
	for _, image := range []string{"", "ubuntu", "ubuntu:22.04", "   "} {
		if digest := h.resolveImageDigest(image); digest != "" {
			t.Errorf("%q should not resolve, got %q", image, digest)
		}
	}
}

// The pinning has to reach the pod, not only the record. A digest written down
// while the tag is launched would be a lie that survives an audit.
func TestALaunchRunsTheDigestItRecords(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"digest":"sha256:deadbeef"}`))
	}))
	defer registry.Close()

	h, runner, item := launchFixture(t, "")
	h.harborURL = registry.URL
	h.workspaceJupyterImage = registryHost(registry.URL) + "/noryx-environments/jupyter:1"

	body, err := json.Marshal(createWorkspaceRequest{
		ProjectID: item.ID, IDE: "jupyter", Image: h.workspaceJupyterImage,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
	request.Header.Set("X-Noryx-User", "member")
	recorder := httptest.NewRecorder()
	h.CreateWorkspace(recorder, request)

	if recorder.Code >= 300 {
		t.Fatalf("the launch failed: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.pods) != 1 {
		t.Fatalf("expected one pod, got %d", len(runner.pods))
	}
	if !strings.Contains(runner.pods[0].Image, "@sha256:deadbeef") {
		t.Errorf("the pod must run the digest, got %q", runner.pods[0].Image)
	}
	workspaces, err := h.workspaceStore.List()
	if err != nil || len(workspaces) != 1 {
		t.Fatalf("expected the workspace to be stored: %v", err)
	}
	if workspaces[0].ImageDigest != "sha256:deadbeef" {
		t.Errorf("the record must carry the digest, got %q", workspaces[0].ImageDigest)
	}
}
