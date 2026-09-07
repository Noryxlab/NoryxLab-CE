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
