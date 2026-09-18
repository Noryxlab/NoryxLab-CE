package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A ref advertisement as a git remote sends it: four hex digits of length, the
// object, the ref name, and capabilities after a NUL on the first line.
func refAdvertisement(refs ...[2]string) string {
	var out strings.Builder
	line := func(payload string) {
		out.WriteString(fmt.Sprintf("%04x%s\n", len(payload)+5, payload))
	}
	line("# service=git-upload-pack")
	for i, ref := range refs {
		payload := ref[0] + " " + ref[1]
		if i == 0 {
			payload += "\x00multi_ack thin-pack side-band agent=git/2.39.0"
		}
		line(payload)
	}
	return out.String()
}

func gitRemote(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") != "git-upload-pack" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

const (
	mainSHA = "2c1f9b0d5e4a7c6b8d9f0a1b2c3d4e5f60718293"
	tagSHA  = "aabbccddeeff00112233445566778899aabbccdd"
)

func TestTheCommitBehindABranchIsResolved(t *testing.T) {
	server := gitRemote(t, refAdvertisement(
		[2]string{mainSHA, "HEAD"},
		[2]string{mainSHA, "refs/heads/main"},
		[2]string{tagSHA, "refs/tags/v1.0"},
	), http.StatusOK)
	defer server.Close()

	// A branch is what somebody types, and it is the value that goes stale.
	if sha := resolveCommitSHA(server.URL, "main"); sha != mainSHA {
		t.Errorf("branch: got %q, want %q", sha, mainSHA)
	}
	if sha := resolveCommitSHA(server.URL, "v1.0"); sha != tagSHA {
		t.Errorf("tag: got %q, want %q", sha, tagSHA)
	}
	if sha := resolveCommitSHA(server.URL, "refs/heads/main"); sha != mainSHA {
		t.Errorf("full ref: got %q, want %q", sha, mainSHA)
	}
	if sha := resolveCommitSHA(server.URL, ""); sha != mainSHA {
		t.Errorf("no ref must mean the default branch: got %q", sha)
	}
}

// An annotated tag advertises the tag object first and the commit it points at
// second. The commit is what was built.
func TestAnAnnotatedTagResolvesToItsCommit(t *testing.T) {
	server := gitRemote(t, refAdvertisement(
		[2]string{"1111111111111111111111111111111111111111", "refs/tags/v2.0"},
		[2]string{mainSHA, "refs/tags/v2.0^{}"},
	), http.StatusOK)
	defer server.Close()

	if sha := resolveCommitSHA(server.URL, "v2.0"); sha != mainSHA {
		t.Errorf("got %q, want the commit %q rather than the tag object", sha, mainSHA)
	}
}

// Every failure means unknown, and unknown is empty. A wrong commit recorded
// in an audit trail is worse than an absent one.
func TestAnUnresolvableCommitIsEmptyRatherThanGuessed(t *testing.T) {
	private := gitRemote(t, "", http.StatusUnauthorized)
	defer private.Close()

	cases := map[string][2]string{
		"a private repository": {private.URL, "main"},
		"an ssh remote":        {"git@github.com:acme/app.git", "main"},
		"an absent ref":        {gitRemote(t, refAdvertisement([2]string{mainSHA, "refs/heads/main"}), http.StatusOK).URL, "release"},
		"an empty repository":  {"", "main"},
	}
	for name, input := range cases {
		if sha := resolveCommitSHA(input[0], input[1]); sha != "" {
			t.Errorf("%s: got %q, want an empty value", name, sha)
		}
	}
}

// A ref that is already a commit is the commit, and the remote is not asked.
func TestARefThatIsAlreadyACommitIsKept(t *testing.T) {
	if sha := resolveCommitSHA("https://example.invalid/acme/app.git", strings.ToUpper(mainSHA)); sha != mainSHA {
		t.Errorf("got %q, want %q", sha, mainSHA)
	}
}
