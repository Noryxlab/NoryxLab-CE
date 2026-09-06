package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Pinning what actually ran.
//
// Every workload recorded its image as `repository:tag`, and a tag moves. We
// move them ourselves: the same tag was pushed twice with different content on
// 2026-09-06. So two runs of the same job could execute different code, and
// nothing anywhere said which one had run - the platform could not answer the
// first question of any incident review or any audit.
//
// A digest cannot move. Resolving it at launch and running `image@sha256:...`
// makes what executed identical to what was written down, which is the whole
// of what "reproducible" can honestly mean about the code.
//
// It does not make the *data* reproducible: datasets in object storage are
// mutable, and recording which dataset was attached is not the same as
// recording what it contained. That needs immutable snapshots and it is a
// separate decision - said out loud rather than implied by the word.

// resolveImageDigest asks the registry what a tag points at right now.
//
// A failure is not fatal: the launch proceeds with the tag, and the record says
// the digest is unknown. A registry hiccup must not stop somebody working, and
// an empty digest is honest where a guessed one would not be.
func (h Handlers) resolveImageDigest(image string) string {
	image = strings.TrimSpace(image)
	if image == "" || strings.Contains(image, "@sha256:") {
		return ""
	}
	registry, repositoryPath, tag, ok := splitImageReference(image)
	if !ok {
		return ""
	}
	// Only our own registry is asked. A public image's digest would be just as
	// useful, but the credentials here are Harbor's and sending them elsewhere
	// is not a thing to do by accident.
	if !strings.EqualFold(registry, registryHost(h.harborURL)) {
		return ""
	}

	// Harbor's own API rather than the registry's.
	//
	// The registry endpoint (/v2/.../manifests/...) speaks the Docker token
	// flow: basic credentials get a 401 and a WWW-Authenticate header, and the
	// client is expected to fetch a bearer token and retry. Asking it with
	// basic auth returns 401 forever - which is what the first version of this
	// did against the real Harbor, silently recording no digest at all while
	// every unit test passed against a fake that accepted basic auth.
	//
	// Harbor's API takes the credentials the platform already holds, and it is
	// the same endpoint the environment catalogue already calls successfully.
	project, repository, ok := splitHarborRepository(repositoryPath)
	if !ok {
		return ""
	}
	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		strings.TrimSuffix(h.harborURL, "/"),
		url.PathEscape(project),
		url.PathEscape(repository),
		url.PathEscape(tag),
	)
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	if user, password := strings.TrimSpace(h.harborUsername), strings.TrimSpace(h.harborPassword); user != "" && password != "" {
		request.SetBasicAuth(user, password)
	}

	response, err := h.harborHTTPClient(5 * time.Second).Do(request)
	if err != nil {
		log.Printf("image digest: cannot reach the registry for %s: %v", image, err)
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("image digest: registry answered %d for %s", response.StatusCode, image)
		return ""
	}
	var artifact struct {
		Digest string `json:"digest"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&artifact); err != nil {
		return ""
	}
	return strings.TrimSpace(artifact.Digest)
}

// splitHarborRepository separates the Harbor project from the repository
// beneath it. Harbor nests repositories under a project, and its API takes the
// two apart - `noryx-environments/noryx-jupyter` is project
// `noryx-environments`, repository `noryx-jupyter`.
func splitHarborRepository(path string) (project, repository string, ok bool) {
	slash := strings.Index(path, "/")
	if slash <= 0 || slash == len(path)-1 {
		return "", "", false
	}
	// A repository deeper than one level is escaped as Harbor expects.
	return path[:slash], strings.ReplaceAll(path[slash+1:], "/", "%252F"), true
}

// pinnedImage is what the pod should run: the digest when we have one, the tag
// otherwise. Written as one function so no caller can record a digest and then
// launch the tag - which would be worse than not resolving it at all.
func pinnedImage(image, digest string) string {
	image = strings.TrimSpace(image)
	digest = strings.TrimSpace(digest)
	if digest == "" || strings.Contains(image, "@sha256:") {
		return image
	}
	repository := image
	if index := strings.LastIndex(image, ":"); index > strings.LastIndex(image, "/") {
		repository = image[:index]
	}
	return repository + "@" + digest
}

// splitImageReference breaks registry.example/project/name:tag apart. It is
// deliberately strict: a reference it does not understand yields no digest
// rather than a guess.
func splitImageReference(image string) (registry, repository, tag string, ok bool) {
	slash := strings.Index(image, "/")
	if slash < 0 {
		return "", "", "", false
	}
	registry = image[:slash]
	// A first segment with no dot and no port is a Docker Hub shorthand, not a
	// registry host.
	if !strings.Contains(registry, ".") && !strings.Contains(registry, ":") {
		return "", "", "", false
	}
	rest := image[slash+1:]
	colon := strings.LastIndex(rest, ":")
	if colon < 0 {
		return registry, rest, "latest", true
	}
	return registry, rest[:colon], rest[colon+1:], true
}

func registryHost(harborURL string) string {
	host := strings.TrimSpace(harborURL)
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	return strings.TrimSuffix(host, "/")
}
