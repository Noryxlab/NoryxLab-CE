package k8s

import (
	"strings"
	"testing"
)

// An installation sizes its own mounts, because the nodes differ by more than
// an order of magnitude between them.
func TestDatasetMountOptionsFollowTheInstallation(t *testing.T) {
	if got := datasetMountOptions(); got != defaultDatasetMountOptions {
		t.Fatalf("with nothing set, got %q, want the default", got)
	}

	custom := "--memory-limit 4096 --stat-cache-ttl 10m --uid 1000 --gid 1000"
	t.Setenv("NORYX_DATASET_MOUNT_OPTIONS", custom)
	if got := datasetMountOptions(); got != custom {
		t.Fatalf("the override was ignored: got %q", got)
	}

	// Blank is not an override. An empty variable is how a deployment carries
	// an unset value, and reading it as "mount with no options at all" would
	// silently drop the uid and gid every workspace needs to read its data.
	t.Setenv("NORYX_DATASET_MOUNT_OPTIONS", "   ")
	if got := datasetMountOptions(); got != defaultDatasetMountOptions {
		t.Fatalf("a blank value became the options: got %q", got)
	}
}

// The default has to stay something any node can run, and has to keep the
// ownership flags: a mount the workspace user cannot read is worse than a slow
// one, and that regression would show up as an unrelated permission error.
func TestDefaultDatasetMountOptionsStayModestAndOwned(t *testing.T) {
	for _, required := range []string{"--uid 1000", "--gid 1000", "--dir-mode", "--file-mode"} {
		if !strings.Contains(defaultDatasetMountOptions, required) {
			t.Fatalf("the default lost %q: %s", required, defaultDatasetMountOptions)
		}
	}
	// No disk cache by default: GeeseFS 0.43.7 cannot bound it, and the
	// filesystem it would fill belongs to the node running the kubelet.
	if strings.Contains(defaultDatasetMountOptions, "--cache ") {
		t.Fatal("the default enables an unbounded disk cache on the node")
	}
}
