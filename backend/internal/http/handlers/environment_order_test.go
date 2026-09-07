package handlers

import (
	"sort"
	"testing"
	"time"
)

// The launch sheet preselects the first environment, so this order is the
// default somebody accepts on every workspace they open. It used to be decided
// by whichever system image had been rebuilt last.
func TestVSCodeLeadsTheEnvironmentList(t *testing.T) {
	now := time.Now().UTC()
	items := []environmentItem{
		{Name: "custom", LatestBuildID: "build-77", UpdatedAt: now},
		{Name: "jupyter", LatestBuildID: "system-jupyter", UpdatedAt: now.Add(-time.Hour)},
		{Name: "rstudio", LatestBuildID: "system-rstudio", UpdatedAt: now.Add(-2 * time.Hour)},
		{Name: "vscode", LatestBuildID: "system-vscode", UpdatedAt: now.Add(-3 * time.Hour)},
	}

	systemRank := map[string]int{"system-vscode": 0, "system-jupyter": 1, "system-rstudio": 2}
	rank := func(item environmentItem) int {
		if position, ok := systemRank[item.LatestBuildID]; ok {
			return position
		}
		return len(systemRank)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if rank(items[i]) != rank(items[j]) {
			return rank(items[i]) < rank(items[j])
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	want := []string{"vscode", "jupyter", "rstudio", "custom"}
	for i, name := range want {
		if items[i].Name != name {
			t.Fatalf("position %d: expected %s, got %s (full order: %v)", i, name, items[i].Name, names(items))
		}
	}
}

func names(items []environmentItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

// An environment that offers both must offer VS Code first, because the launch
// takes the first entry when nobody picks.
func TestAPythonEnvironmentOffersVSCodeFirst(t *testing.T) {
	ides := deriveWorkspaceIDEs("harbor.lan/noryx-environments/noryx-python:1.0")
	if len(ides) == 0 || ides[0] != "vscode" {
		t.Errorf("expected vscode first, got %v", ides)
	}
}
