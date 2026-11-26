package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Asking any OCI registry what a tag points at.
//
// This used to go through Harbor's own API, for a good reason that has since
// become a cost. The reason: the registry endpoint speaks the Docker token
// flow - basic credentials get a 401 with a WWW-Authenticate header, and the
// caller is expected to fetch a bearer token and retry. The first version of
// this sent basic auth and took the 401 as "no digest", silently recording
// nothing while every unit test passed against a fake that accepted basic
// auth. Harbor's API, which does take basic credentials, made the problem go
// away.
//
// The cost is that it made the platform's provenance a Harbor feature. On
// ECR, Artifact Registry or ACR the call returns nothing, so a run records no
// digest at launch - on exactly the registries a client arrives with. Since an
// installation on somebody else's cloud is now a thing we intend to do, the
// honest fix is to speak the protocol every registry speaks, including Harbor.
//
// So the token flow is implemented rather than avoided, and this one path
// serves every registry. What stays Harbor-specific is enrichment the standard
// does not cover: an artifact's size, its vulnerability counts, and deleting a
// built image.
const registryCallTimeout = 6 * time.Second

// Every manifest shape a registry may answer with. Omitting the index types is
// how a multi-architecture image yields a 404 from a registry that would
// happily have served it.
var registryManifestAccept = strings.Join([]string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.docker.distribution.manifest.v2+json",
}, ", ")

// digestFromRegistry resolves one tag against the registry the platform holds
// credentials for.
//
// It answers "" for anything it cannot establish. A launch proceeds on the tag
// and records no digest, which is honest; a guessed digest would not be.
func (h Handlers) digestFromRegistry(repository, reference string) string {
	base := strings.TrimRight(strings.TrimSpace(h.harborURL), "/")
	if base == "" || repository == "" || reference == "" {
		return ""
	}
	// Path-escaped per segment: a repository is `project/name` and the
	// slashes are part of the path, not something to encode away.
	endpoint := base + "/v2/" + escapeRepository(repository) + "/manifests/" + url.PathEscape(reference)

	response, body, ok := h.registryGet(endpoint, "")
	if !ok {
		return ""
	}
	if response.StatusCode == http.StatusUnauthorized {
		token := h.registryBearerToken(response.Header.Get("Www-Authenticate"))
		if token == "" {
			log.Printf("image digest: %s asked for a bearer token and would not issue one", repository)
			return ""
		}
		response, body, ok = h.registryGet(endpoint, token)
		if !ok {
			return ""
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("image digest: registry answered %d for %s:%s", response.StatusCode, repository, reference)
		return ""
	}

	// The header when the registry sends it, the manifest's own hash
	// otherwise. The second is not a fallback in the weak sense: a manifest's
	// digest *is* the sha256 of those exact bytes, so computing it is as
	// authoritative as being told, and it keeps registries that omit the
	// header from silently costing us provenance.
	if digest := strings.TrimSpace(response.Header.Get("Docker-Content-Digest")); digest != "" {
		return digest
	}
	if len(body) == 0 {
		return ""
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// registryGet performs one attempt, with basic credentials when no bearer
// token is supplied and the bearer token when it is.
//
// GET rather than HEAD, deliberately: a HEAD saves a few kilobytes and loses
// the body that the digest can be computed from when a registry omits the
// header. Manifests are small.
func (h Handlers) registryGet(endpoint, bearer string) (*http.Response, []byte, bool) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, false
	}
	request.Header.Set("Accept", registryManifestAccept)
	switch {
	case bearer != "":
		request.Header.Set("Authorization", "Bearer "+bearer)
	default:
		// Sent on the first attempt because some registries - ECR among them -
		// take basic credentials directly and never ask for a token.
		if user, password := strings.TrimSpace(h.harborUsername), strings.TrimSpace(h.harborPassword); user != "" && password != "" {
			request.SetBasicAuth(user, password)
		}
	}
	response, err := h.harborHTTPClient(registryCallTimeout).Do(request)
	if err != nil {
		log.Printf("image digest: cannot reach the registry: %v", err)
		return nil, nil, false
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, nil, false
	}
	return response, body, true
}

// registryBearerToken walks the Docker token flow from the challenge the
// registry answered a 401 with.
//
// The challenge names where to ask and what to ask for; asking for a different
// scope returns a token that authorises nothing, which reads as a 401 on the
// retry. So the parameters are taken from the challenge rather than built.
func (h Handlers) registryBearerToken(challenge string) string {
	realm, parameters := parseAuthenticateChallenge(challenge)
	if realm == "" {
		return ""
	}
	endpoint, err := url.Parse(realm)
	if err != nil {
		return ""
	}
	query := endpoint.Query()
	for _, name := range []string{"service", "scope"} {
		if value := parameters[name]; value != "" {
			query.Set(name, value)
		}
	}
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ""
	}
	if user, password := strings.TrimSpace(h.harborUsername), strings.TrimSpace(h.harborPassword); user != "" && password != "" {
		request.SetBasicAuth(user, password)
	}
	response, err := h.harborHTTPClient(registryCallTimeout).Do(request)
	if err != nil {
		log.Printf("image digest: token endpoint unreachable: %v", err)
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	// Registries disagree on the field name and agree on the meaning.
	var issued struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&issued); err != nil {
		return ""
	}
	if token := strings.TrimSpace(issued.Token); token != "" {
		return token
	}
	return strings.TrimSpace(issued.AccessToken)
}

// parseAuthenticateChallenge reads `Bearer realm="...",service="...",scope="..."`.
//
// Only the Bearer scheme is understood. A registry answering Basic has already
// been given basic credentials on the first attempt, so there is nothing left
// to try.
func parseAuthenticateChallenge(header string) (realm string, parameters map[string]string) {
	parameters = map[string]string{}
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", parameters
	}
	for _, part := range splitChallengeParameters(header[len("bearer "):]) {
		equals := strings.Index(part, "=")
		if equals <= 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(part[:equals]))
		value := strings.Trim(strings.TrimSpace(part[equals+1:]), `"`)
		if name == "realm" {
			realm = value
			continue
		}
		parameters[name] = value
	}
	return realm, parameters
}

// splitChallengeParameters splits on commas that are not inside quotes: a
// scope legitimately contains commas when a registry asks for several.
func splitChallengeParameters(value string) []string {
	parts := []string{}
	quoted := false
	current := strings.Builder{}
	for _, char := range value {
		switch {
		case char == '"':
			quoted = !quoted
			current.WriteRune(char)
		case char == ',' && !quoted:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(char)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// escapeRepository escapes each path segment and keeps the separators. A
// repository is `project/name` or deeper, and the slashes belong to the URL.
func escapeRepository(repository string) string {
	segments := strings.Split(strings.Trim(repository, "/"), "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
