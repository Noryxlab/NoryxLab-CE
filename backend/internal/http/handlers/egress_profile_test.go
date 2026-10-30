package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A workspace over regulated data has no route to the internet, and one over
// ordinary data keeps the access it had.
//
// This is the control Essilor asked for on 2026-10-30, after finding that a
// file could be downloaded from a workspace with a right click. The buttons
// were never the exfiltration path: the pod carries a shell, curl and sudo,
// and the namespace admitted every public address.
func TestRegulatedDataClosesTheWorkloadsEgress(t *testing.T) {
	handlers, regulated, ordinary := projectsWithAndWithoutRegulatedData(t)

	for _, item := range []struct {
		enforcement string
		project     string
		expected    string
	}{
		{settings.EgressEnforcementRegulated, regulated, "hds-regulated"},
		{settings.EgressEnforcementRegulated, ordinary, "unrestricted"},
		{settings.EgressEnforcementStrict, regulated, "hds-regulated"},
		{settings.EgressEnforcementStrict, ordinary, "isolated"},
		{settings.EgressEnforcementOff, regulated, "unrestricted"},
		{settings.EgressEnforcementOff, ordinary, "unrestricted"},
	} {
		if err := handlers.settings.Set(settings.KeyEgressEnforcement, item.enforcement, "test"); err != nil {
			t.Fatal(err)
		}
		if profile := handlers.workloadEgressProfile(item.project); profile != item.expected {
			t.Fatalf("enforcement %q on project %q: expected %q, got %q",
				item.enforcement, item.project, item.expected, profile)
		}
	}
}

// A workload with no project is the platform's own helper, and it is closed
// whatever the installation decided.
func TestAWorkloadWithNoProjectIsClosed(t *testing.T) {
	handlers, _, _ := projectsWithAndWithoutRegulatedData(t)
	if err := handlers.settings.Set(settings.KeyEgressEnforcement, settings.EgressEnforcementOff, "test"); err != nil {
		t.Fatal(err)
	}
	if profile := handlers.workloadEgressProfile("  "); profile != "isolated" {
		t.Fatalf("expected isolated, got %q", profile)
	}
}

// An unreadable setting leaves the regulated rule in force rather than
// falling open: a value nobody recognises is not a decision to open the door.
func TestAnUnknownEnforcementValueKeepsTheRegulatedRule(t *testing.T) {
	handlers, regulated, ordinary := projectsWithAndWithoutRegulatedData(t)
	if err := handlers.settings.Set(settings.KeyEgressEnforcement, settings.EgressEnforcementOff, "test"); err != nil {
		t.Fatal(err)
	}
	// Written behind the registry's back, as a hand-edited row would be.
	handlers.settings = settingsWithRawValue(t, settings.KeyEgressEnforcement, "yes-please")
	if profile := handlers.workloadEgressProfile(regulated); profile != "hds-regulated" {
		t.Fatalf("expected hds-regulated, got %q", profile)
	}
	if profile := handlers.workloadEgressProfile(ordinary); profile != "unrestricted" {
		t.Fatalf("expected unrestricted, got %q", profile)
	}
}

// The label the policies select on is the one the profile named.
func TestTheProfileReachesTheLabels(t *testing.T) {
	labels := withEgressProfile(map[string]string{"app.kubernetes.io/name": "noryx-workspace"}, "hds-regulated")
	if labels["noryx.io/egress-profile"] != "hds-regulated" {
		t.Fatalf("unexpected labels %v", labels)
	}
	if labels["app.kubernetes.io/name"] != "noryx-workspace" {
		t.Fatal("the caller's own labels must survive")
	}
	if _, named := withEgressProfile(nil, "  ")["noryx.io/egress-profile"]; named {
		t.Fatal("an empty profile names nothing; the runtime decides the default")
	}
}

func projectsWithAndWithoutRegulatedData(t *testing.T) (Handlers, string, string) {
	t.Helper()
	datasets := memory.NewDatasetStore()
	regulated := dataset.New("stef", "HDS-For-premyom1000", "", "hds-for", "", "s3", "hds", "https://example.invalid", "")
	ordinary := dataset.New("stef", "open-data", "", "nonhds-open", "", "s3", "non-hds", "https://example.invalid", "")
	for _, item := range []dataset.Dataset{regulated, ordinary} {
		if err := datasets.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	resources := memory.NewProjectResourceStore()
	if err := resources.AttachDataset("project-regulated", regulated.ID); err != nil {
		t.Fatal(err)
	}
	if err := resources.AttachDataset("project-ordinary", ordinary.ID); err != nil {
		t.Fatal(err)
	}
	return Handlers{
		datasetStore:         datasets,
		projectResourceStore: resources,
		settings:             settingsForTest(t),
	}, "project-regulated", "project-ordinary"
}

func settingsForTest(t *testing.T) *settings.Resolver {
	t.Helper()
	return settings.NewResolver(memory.NewSettingsStore())
}

// settingsWithRawValue writes a value the registry would have refused, which
// is what a hand-edited row in the database looks like.
func settingsWithRawValue(t *testing.T, key, value string) *settings.Resolver {
	t.Helper()
	store := memory.NewSettingsStore()
	if err := store.Set(key, value, "test"); err != nil {
		t.Fatal(err)
	}
	return settings.NewResolver(store)
}
