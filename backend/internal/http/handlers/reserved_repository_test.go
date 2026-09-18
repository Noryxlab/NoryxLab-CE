package handlers

import "testing"

// A project may not build into a repository the platform runs from.
//
// On 2026-09-07 a build named the platform's own noryx-vscode repository.
// Catalogue entries are keyed on the repository, so it merged with the system
// environment and became the image the list offered, while the platform went
// on starting the reference it is configured with. Every launch from that
// entry was then refused for an image nobody could see had been substituted.
func TestThePlatformsOwnRepositoriesAreRefused(t *testing.T) {
	h := Handlers{
		workspaceVSCodeImage:  "harbor.example/noryx-environments/noryx-vscode:0.1.2",
		workspaceJupyterImage: "harbor.example/noryx-environments/noryx-jupyter:0.1.0",
	}
	for _, attempt := range []string{
		"harbor.example/noryx-environments/noryx-vscode",
		// A different tag is the case that actually happened, and the one that
		// looks harmless: same repository, so the same merge.
		"harbor.example/noryx-environments/noryx-vscode:1788812980",
		"harbor.example/noryx-environments/noryx-jupyter:anything",
	} {
		if got := h.reservedRepositoryFor(attempt); got == "" {
			t.Errorf("%s was allowed into a platform repository", attempt)
		}
	}
}

func TestAProjectsOwnNamesArePermitted(t *testing.T) {
	// Beside the platform's environments, not inside them. The derived form
	// carries the project in the repository name, which is why it is safe.
	h := Handlers{workspaceVSCodeImage: "harbor.example/noryx-environments/noryx-vscode:0.1.2"}
	for _, attempt := range []string{
		"harbor.example/noryx-environments/1cf6b27911-my-env:r1",
		"harbor.example/noryx-environments/noryx-vscode-fork:r1",
		"harbor.example/somewhere-else/noryx-vscode:0.1.2",
		"",
	} {
		if got := h.reservedRepositoryFor(attempt); got != "" {
			t.Errorf("%s was refused as %q, but it is not a platform repository", attempt, got)
		}
	}
}

// The same reservation guards deletion, and it did not always.
//
// Environment deletion compared three exact references - the Community images
// at their configured tags. A kind registered by a module was not among them,
// so on an installation where imaging is done, the image that discipline works
// in could be deleted from the registry by anybody holding the role on any
// project. Comparing repositories rather than references also covers a rebuild
// pushed beside a platform image, which matched nothing and was therefore
// deletable while sitting under the platform's own name.
func TestDeletionIsRefusedForEveryPlatformRepository(t *testing.T) {
	h := Handlers{
		workspaceVSCodeImage:  "harbor.example/noryx-environments/noryx-vscode:0.1.2",
		workspaceJupyterImage: "harbor.example/noryx-environments/noryx-jupyter:0.1.0",
		workspaceRStudioImage: "harbor.example/noryx-environments/noryx-rstudio:0.1.0",
	}
	for _, protected := range []string{
		"harbor.example/noryx-environments/noryx-vscode:0.1.2",
		// The tag the platform is not configured with, in the repository it
		// runs from: deletable before, because only references were compared.
		"harbor.example/noryx-environments/noryx-vscode:1788812980",
		"harbor.example/noryx-environments/noryx-jupyter:0.1.0",
	} {
		if h.reservedRepositoryFor(protected) == "" {
			t.Errorf("%s would be deleted from the registry", protected)
		}
	}
	// A project's own environment stays deletable: that is the whole point of
	// being able to delete one.
	if h.reservedRepositoryFor("harbor.example/noryx-environments/1cf6b279-114-test-stef") != "" {
		t.Error("a project's own environment was protected, which makes cleanup impossible")
	}
}
