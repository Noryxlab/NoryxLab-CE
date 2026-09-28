package k8s

import (
	"encoding/json"
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// The boundary is a mount, not a convention.
//
// A cohort mounted as links over the dataset is a view: the bucket is in the
// same container, and a shell walks out of the selection into everything.
// Moving the dataset into a sidecar makes the kernel the thing that refuses -
// a Kubernetes volume is declared on the pod and mounted per container, so the
// workspace can be given the cache and denied the bucket.
//
// Measured on EMSE on 2026-09-26 with two hand-built containers: `/datasets`
// answered "No such file or directory" from the workshop, and the cache was a
// read-only filesystem. This is that arrangement, produced by the runtime.
func TestTheDatasetReachesTheFillerAndNotTheWorkspace(t *testing.T) {
	payload := podPayload(noryxruntime.PodSpec{
		PodName: "wks-1", Image: "harbor/vscode:1",
		Volumes: []noryxruntime.PersistentVolumeClaimMount{
			{ClaimName: "cache", MountPath: "/home/onyxia/work/cohorts", ReadOnly: true},
		},
		Sidecar: &noryxruntime.SidecarSpec{
			Name: "cohort-filler", Image: "harbor/vscode:1",
			Volumes: []noryxruntime.PersistentVolumeClaimMount{
				{ClaimName: "dataset-hds-for", MountPath: "/datasets/hds-for", ReadOnly: true},
				{ClaimName: "cache", MountPath: "/cache"},
			},
		},
	})

	spec := payload["spec"].(map[string]any)
	containers := spec["containers"].([]map[string]any)
	if len(containers) != 2 {
		t.Fatalf("expected the workspace and its filler, got %d container(s)", len(containers))
	}

	main, _ := json.Marshal(containers[0]["volumeMounts"])
	if strings.Contains(string(main), "/datasets/") {
		t.Fatalf("the workspace must not mount the dataset: %s", main)
	}
	if !strings.Contains(string(main), "cohorts") {
		t.Fatalf("the workspace must mount the cache: %s", main)
	}

	filler, _ := json.Marshal(containers[1]["volumeMounts"])
	if !strings.Contains(string(filler), "/datasets/hds-for") || !strings.Contains(string(filler), "/cache") {
		t.Fatalf("the filler needs both the dataset and the cache: %s", filler)
	}

	// And the shared claim is declared once, or the two containers would be
	// handed two different volumes with the same name.
	volumes, _ := json.Marshal(spec["volumes"])
	if strings.Count(string(volumes), `"claimName":"cache"`) != 1 {
		t.Fatalf("the cache claim must be declared exactly once: %s", volumes)
	}
}

// No sidecar asked for, no sidecar built: every workspace launched before this
// existed must produce exactly the pod it produced yesterday.
func TestWithoutASidecarThePodIsUnchanged(t *testing.T) {
	payload := podPayload(noryxruntime.PodSpec{PodName: "wks-1", Image: "harbor/vscode:1"})
	containers := payload["spec"].(map[string]any)["containers"].([]map[string]any)
	if len(containers) != 1 || containers[0]["name"] != "main" {
		t.Fatalf("expected the single main container, got %v", containers)
	}
}
