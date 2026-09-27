package handlers

import (
	"strings"
	"testing"
)

// A clone that fails must leave evidence.
//
// Both branches ended in "|| true", which keeps the workspace starting when a
// repository is unreachable - the right call - while throwing away the reason.
// Azure DevOps cloning was reported broken with nothing to diagnose from: no
// registered repository on either cluster, and fourteen days of logs with no
// trace of the attempt.
func TestAFailedCloneIsAnnounced(t *testing.T) {
	for _, repo := range []workspaceAttachedRepo{
		{Name: "etudes", URL: "https://dev.azure.com/org/project/_git/etudes"},
		{Name: "etudes", URL: "https://dev.azure.com/org/project/_git/etudes", AuthEnvName: "REPO_TOKEN"},
	} {
		script := strings.Join(repositoryBootstrapLines(repo, "/repos/etudes"), "\n")
		if strings.Contains(script, "|| true") {
			t.Fatal("a clone must not fail silently")
		}
		if !strings.Contains(script, "CLONE FAILED for etudes") {
			t.Fatal("the failure must name the repository")
		}
		if !strings.Contains(script, "dev.azure.com/org/project/_git/etudes") {
			t.Fatal("the failure must name the address, which is what a reporter copies")
		}
	}
}

// And it tries once more without the shortcut, because the shortcut is ours.
func TestAShallowCloneFallsBackToAFullOne(t *testing.T) {
	script := strings.Join(repositoryBootstrapLines(
		workspaceAttachedRepo{Name: "etudes", URL: "https://dev.azure.com/org/p/_git/etudes"},
		"/repos/etudes"), "\n")
	shallow := strings.Index(script, "clone --depth 1")
	full := strings.LastIndex(script, "git clone '")
	if shallow < 0 || full < 0 || full < shallow {
		t.Fatalf("a failed shallow clone must be retried in full; got:\n%s", script)
	}
}
