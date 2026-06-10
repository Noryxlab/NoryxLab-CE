package handlers

import "testing"

func TestAddSystemEnvironmentSupportsGlobalCatalog(t *testing.T) {
	items := map[string]*environmentItem{}
	addSystemEnvironment(items, "", "harbor.lan/noryx-environments/noryx-vscode:0.1.0", systemEnvironmentDefinitions["system-vscode"])

	item, ok := items["|harbor.lan/noryx-environments/noryx-vscode:0.1.0"]
	if !ok {
		t.Fatal("expected system environment in global catalog")
	}
	if item.ProjectID != "" || item.Category != "system" || item.LatestBuildID != "system-vscode" {
		t.Fatalf("unexpected global system environment: %#v", item)
	}
}
