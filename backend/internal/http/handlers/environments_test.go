package handlers

import "testing"

func TestAddSystemEnvironmentSupportsGlobalCatalog(t *testing.T) {
	items := map[string]*environmentItem{}
	addSystemEnvironment(items, "", "harbor.example.local/noryx-environments/noryx-vscode:0.1.1", systemEnvironmentDefinitions["system-vscode"])

	item, ok := items["|harbor.example.local/noryx-environments/noryx-vscode:0.1.1"]
	if !ok {
		t.Fatal("expected system environment in global catalog")
	}
	if item.ProjectID != "" || item.Category != "system" || item.LatestBuildID != "system-vscode" {
		t.Fatalf("unexpected global system environment: %#v", item)
	}
}
