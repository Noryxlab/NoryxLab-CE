package handlers

import "testing"

// A rebuild is a revision, not a replacement.
//
// Entries are keyed on the repository rather than the reference so that
// rebuilding an environment does not create a second one. That is right, and
// it let a rebuild quietly become the offered environment: a build pushed
// noryx-vscode:1788812980 to the platform's own repository, merged with the
// system entry, and the merged row showed the system name over the build's
// image. The list read 0.1.2, the launch carried the timestamp, and the
// platform refused an image nobody could see had been substituted.
func TestASystemEnvironmentKeepsItsOwnImage(t *testing.T) {
	const system = "harbor.example/noryx-environments/noryx-vscode:0.1.2"
	const rebuild = "harbor.example/noryx-environments/noryx-vscode:1788812980"

	items := map[string]*environmentItem{
		"|harbor.example/noryx-environments/noryx-vscode": {
			ID:               "|harbor.example/noryx-environments/noryx-vscode",
			Name:             "noryx-vscode:1788812980",
			DestinationImage: rebuild,
			WorkspaceIDEs:    []string{"jupyter", "vscode"},
			Revisions:        []environmentRevision{{BuildID: "b1", DestinationImage: rebuild}},
		},
	}
	addSystemEnvironment(items, "", system, systemEnvironmentDefinitions["system-vscode"])

	merged := items["|harbor.example/noryx-environments/noryx-vscode"]
	if merged.DestinationImage != system {
		t.Errorf("a launch would carry %q; the platform runs %q", merged.DestinationImage, system)
	}
	if merged.WorkspaceIDEs[0] != "vscode" {
		t.Errorf("the system kind is not first: %v", merged.WorkspaceIDEs)
	}
	// The rebuild is history, not a loss: it stays where the history of an
	// environment belongs.
	found := false
	for _, revision := range merged.Revisions {
		if revision.DestinationImage == rebuild {
			found = true
		}
	}
	if !found {
		t.Error("the rebuild disappeared from the revisions")
	}
}

func TestAnUntouchedSystemEnvironmentIsUnchanged(t *testing.T) {
	// The ordinary case still works: nothing merged, the platform's image and
	// its single kind.
	const system = "harbor.example/noryx-environments/noryx-jupyter:0.1.0"
	items := map[string]*environmentItem{}
	addSystemEnvironment(items, "", system, systemEnvironmentDefinitions["system-jupyter"])

	entry := items["|harbor.example/noryx-environments/noryx-jupyter"]
	if entry == nil || entry.DestinationImage != system {
		t.Fatalf("system environment not registered: %+v", entry)
	}
	if len(entry.WorkspaceIDEs) != 1 || entry.WorkspaceIDEs[0] != "jupyter" {
		t.Errorf("kinds: %v", entry.WorkspaceIDEs)
	}
}
