package handlers

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What a build was made from, and what it produced.
//
// A build record held a git ref and a tag, and neither is stable: `main` names
// a different commit every week, and a tag can be overwritten in the registry.
// So the platform could show that an environment had been built and could not
// say from what, or that two runs a month apart had used the same code. That
// is a gap in an audit trail before it is a gap in a feature.
//
// Both halves are resolved opportunistically and never invented. A private
// repository the platform cannot read anonymously leaves the commit empty, and
// empty is displayed as unknown rather than smoothed over.

var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// resolveCommitSHA asks the remote which commit a ref points at, using git's
// smart HTTP ref discovery - the same request `git ls-remote` makes. No git
// binary, no clone, no working copy: one HTTPS GET against a repository the
// platform is about to hand to the builder anyway.
func resolveCommitSHA(repository, ref string) string {
	repository = strings.TrimSpace(repository)
	lower := strings.ToLower(repository)
	if repository == "" || !(strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) {
		// ssh:// and git@ remotes need a key the platform does not hold, and
		// guessing at one would turn a missing value into a wrong one. Plain
		// http is allowed because a self-hosted forge on a private network is
		// a real deployment and this request carries no credential: it reads
		// the ref advertisement the remote publishes and nothing else.
		return ""
	}
	if commitSHAPattern.MatchString(strings.ToLower(strings.TrimSpace(ref))) {
		// Already a commit. Asking the remote would only confirm it.
		return strings.ToLower(strings.TrimSpace(ref))
	}
	endpoint := strings.TrimSuffix(repository, "/") + "/info/refs?service=git-upload-pack"
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("User-Agent", "git/2.0 (noryx-ce)")

	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		log.Printf("build provenance: cannot reach %s to resolve %q: %v", repository, ref, err)
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// 401 on a private repository is the expected case, not a defect.
		log.Printf("build provenance: %s answered HTTP %d for ref discovery", repository, response.StatusCode)
		return ""
	}

	candidates := candidateRefNames(ref)
	head, match := "", ""
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 4<<20))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		sha, name, peeled, ok := parsePktRefLine(scanner.Text())
		if !ok {
			continue
		}
		if name == "HEAD" && head == "" {
			head = sha
		}
		for _, candidate := range candidates {
			if name != candidate {
				continue
			}
			// An annotated tag is advertised twice: the tag object, then the
			// commit it points at, suffixed ^{}. Taking the first match would
			// record the tag object as the source of the build - a valid
			// object that is not a commit and cannot be checked out.
			if peeled {
				return sha
			}
			if match == "" {
				match = sha
			}
		}
	}
	if match != "" {
		return match
	}
	// No ref was asked for, so the default branch is the honest answer.
	if strings.TrimSpace(ref) == "" {
		return head
	}
	return ""
}

// parsePktRefLine reads one line of a smart HTTP ref advertisement. Each line
// is prefixed with a four-digit length and may carry capabilities after a NUL,
// both of which are stripped here. peeled marks the ^{} form, which is the
// commit behind an annotated tag. Lines that are not refs yield ok=false.
func parsePktRefLine(line string) (sha, name string, peeled, ok bool) {
	if len(line) > 4 {
		// The length prefix is hex and always four characters; dropping it
		// blindly would corrupt a line that does not carry one.
		if _, err := strconv.ParseUint(line[:4], 16, 32); err == nil {
			line = line[4:]
		}
	}
	if index := strings.IndexByte(line, 0); index >= 0 {
		line = line[:index]
	}
	line = strings.TrimSpace(line)
	sha, name, found := strings.Cut(line, " ")
	if !found || !commitSHAPattern.MatchString(sha) {
		return "", "", false, false
	}
	name = strings.TrimSpace(name)
	if trimmed := strings.TrimSuffix(name, "^{}"); trimmed != name {
		name, peeled = trimmed, true
	}
	if name == "" {
		return "", "", false, false
	}
	return sha, name, peeled, true
}

// candidateRefNames turns what somebody typed into the full ref names a remote
// advertises. "main", "refs/heads/main" and a tag all reach the same place.
func candidateRefNames(ref string) []string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return []string{"HEAD"}
	}
	if strings.HasPrefix(ref, "refs/") {
		return []string{ref}
	}
	return []string{
		"refs/heads/" + ref,
		"refs/tags/" + ref,
		ref,
	}
}
