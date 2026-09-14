package handlers

import "testing"

func TestAddSystemEnvironmentSupportsGlobalCatalog(t *testing.T) {
	items := map[string]*environmentItem{}
	addSystemEnvironment(items, "", "harbor.example.local/noryx-environments/noryx-vscode:0.1.2", systemEnvironmentDefinitions["system-vscode"])

	// Keyed by repository, exactly as a build is: keying this one on the full
	// reference while builds keyed on the repository produced three rows for
	// the same environment.
	item, ok := items["|harbor.example.local/noryx-environments/noryx-vscode"]
	if !ok {
		t.Fatalf("expected system environment in global catalog, got keys %v", keysOf(items))
	}
	if item.ProjectID != "" || item.Category != "system" || item.LatestBuildID != "system-vscode" {
		t.Fatalf("unexpected global system environment: %#v", item)
	}
}

func keysOf(items map[string]*environmentItem) []string {
	out := make([]string, 0, len(items))
	for key := range items {
		out = append(out, key)
	}
	return out
}

// Rebuilding a system image must produce a revision, not a second row.
//
// A system environment is platform-wide and carries no project; a build
// carries the project that produced it. Keyed the same way, the two never met:
// the catalogue showed noryx-vscode:0.1.2 twice - once from the system
// definition and once from the rebuild - and grew another row on every
// rebuild, where the honest answer is one environment with a history.
func TestRebuildingASystemImageAddsARevisionRatherThanARow(t *testing.T) {
	const image = "harbor.example.local/noryx-environments/noryx-vscode:0.1.2"
	const repository = "harbor.example.local/noryx-environments/noryx-vscode"

	items := map[string]*environmentItem{}
	// What the build loop produces for a project that rebuilt the system image,
	// with the key the handler now computes for it.
	items["|"+repository] = &environmentItem{
		ID:               "|" + repository,
		ProjectID:        "projet-de-cedric",
		Name:             "noryx-vscode:0.1.2",
		Category:         "custom",
		WorkspaceIDEs:    []string{"vscode", "jupyter"},
		DestinationImage: image,
		Revisions:        []environmentRevision{{BuildID: "build-1", Status: "succeeded"}},
	}

	addSystemEnvironment(items, "", image, systemEnvironmentDefinitions["system-vscode"])

	if len(items) != 1 {
		t.Fatalf("the catalogue holds %d rows for one environment: %v", len(items), keysOf(items))
	}
	item := items["|"+repository]
	if item.Category != "system" {
		t.Fatalf("category = %q: a rebuild of a platform image is still a platform image", item.Category)
	}
	if len(item.Revisions) != 2 {
		t.Fatalf("revisions = %d, want the system definition and the rebuild", len(item.Revisions))
	}
	// The system definition is the newest: it is what the platform runs by
	// default, and the list shows the latest first.
	if item.Revisions[0].BuildID != "system-vscode" {
		t.Fatalf("first revision = %q", item.Revisions[0].BuildID)
	}
}
