package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// The registry protocol, which every registry speaks - including the ones a
// client arrives with.
func TestTheDigestComesFromTheRegistry(t *testing.T) {
	var asked, accepted string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.Method + " " + r.URL.Path
		accepted = r.Header.Get("Accept")
		w.Header().Set("Docker-Content-Digest", "sha256:1234")
		_, _ = w.Write([]byte(`{"schemaVersion":2}`))
	}))
	defer registry.Close()

	h := Handlers{harborURL: registry.URL}
	digest := h.resolveImageDigest(registryHost(registry.URL) + "/noryx-environments/jupyter:0.1.0")

	if digest != "sha256:1234" {
		t.Fatalf("digest = %q, want sha256:1234", digest)
	}
	if asked != "GET /v2/noryx-environments/jupyter/manifests/0.1.0" {
		t.Errorf("asked %q", asked)
	}
	// A multi-architecture image is an index, and a request that does not
	// accept one is answered 404 by a registry that holds it.
	if !strings.Contains(accepted, "image.index.v1+json") {
		t.Errorf("Accept did not offer the index types: %q", accepted)
	}
}

// The flow Harbor, Artifact Registry and ACR all use: basic credentials get a
// 401 and a challenge, and the caller fetches a token and retries. The first
// version of this treated the 401 as "no digest" and recorded nothing in
// production while passing against a fake that accepted basic auth.
func TestTheBearerChallengeIsFollowed(t *testing.T) {
	var scope, service, realm string
	var attempts int
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			scope, service = r.URL.Query().Get("scope"), r.URL.Query().Get("service")
			_, _ = w.Write([]byte(`{"token":"issued-token"}`))
			return
		}
		attempts++
		if r.Header.Get("Authorization") != "Bearer issued-token" {
			w.Header().Set("Www-Authenticate",
				`Bearer realm="`+realm+`/token",service="harbor",scope="repository:noryx-environments/jupyter:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Docker-Content-Digest", "sha256:beef")
		_, _ = w.Write([]byte(`{"schemaVersion":2}`))
	}))
	defer registry.Close()
	realm = registry.URL

	h := Handlers{harborURL: registry.URL, harborUsername: "robot", harborPassword: "secret"}
	if digest := h.resolveImageDigest(registryHost(registry.URL) + "/noryx-environments/jupyter:0.1.0"); digest != "sha256:beef" {
		t.Fatalf("digest = %q, want sha256:beef", digest)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want one refused and one with the token", attempts)
	}
	// Taken from the challenge, not built: a token issued for a scope we
	// invented authorises nothing, and reads as a second 401.
	if scope != "repository:noryx-environments/jupyter:pull" || service != "harbor" {
		t.Errorf("asked for scope %q service %q", scope, service)
	}
}

// A registry that omits the header still has to yield a digest: a manifest's
// digest is the sha256 of those exact bytes, so computing it is as
// authoritative as being told.
func TestADigestIsComputedWhenTheHeaderIsMissing(t *testing.T) {
	body := `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer registry.Close()

	sum := sha256.Sum256([]byte(body))
	want := "sha256:" + hex.EncodeToString(sum[:])

	h := Handlers{harborURL: registry.URL}
	if digest := h.resolveImageDigest(registryHost(registry.URL) + "/noryx-environments/jupyter:0.1.0"); digest != want {
		t.Fatalf("digest = %q, want %q", digest, want)
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
		w.Header().Set("Docker-Content-Digest", "sha256:deadbeef")
		_, _ = w.Write([]byte(`{"schemaVersion":2}`))
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
