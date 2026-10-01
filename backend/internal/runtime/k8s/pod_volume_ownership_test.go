package k8s

import (
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// A non-root container has to be able to write to a volume nobody has written
// to yet.
//
// The shipped images run as noryx, 1000:1000, and a freshly provisioned volume
// belongs to root. Mounted over the profile directory that made the workspace
// bootstrap die on its first write - "Permission denied", exit 2 - before the
// IDE started. fsGroup is what the kubelet reads to fix the ownership.
func TestAVolumeIsGivenToTheGroupTheContainerRunsAs(t *testing.T) {
	payload := podPayload(noryxruntime.PodSpec{
		PodName: "wks-1", Image: "harbor/vscode:1", FSGroup: 1000,
		Volumes: []noryxruntime.PersistentVolumeClaimMount{
			{ClaimName: "profile-alice", MountPath: "/home/noryx/.noryx-profile"},
		},
	})
	contexte, ok := payload["spec"].(map[string]any)["securityContext"].(map[string]any)
	if !ok {
		t.Fatal("a pod with an fsGroup must carry a security context")
	}
	if contexte["fsGroup"] != int64(1000) {
		t.Fatalf("fsGroup = %v, want 1000", contexte["fsGroup"])
	}
	// Sinon le kubelet rechown recursivement a chaque montage, et un volume de
	// projet contient une etude : le workspace attendrait le parcours de tous
	// les fichiers pour reparer une appartenance deja correcte.
	if contexte["fsGroupChangePolicy"] != "OnRootMismatch" {
		t.Fatalf("fsGroupChangePolicy = %v, want OnRootMismatch", contexte["fsGroupChangePolicy"])
	}
}

// Et sans fsGroup, aucun contexte de securite : un pod qui n'a pas de volume
// n'a pas besoin qu'on touche a son appartenance.
func TestAPodWithoutAnFSGroupCarriesNoSecurityContext(t *testing.T) {
	payload := podPayload(noryxruntime.PodSpec{PodName: "job-1", Image: "harbor/python:1"})
	if _, present := payload["spec"].(map[string]any)["securityContext"]; present {
		t.Fatal("a pod without an fsGroup must not declare a security context")
	}
}
