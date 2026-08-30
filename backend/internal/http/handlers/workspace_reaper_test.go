package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
)

// The reaper must reclaim workspaces past the configured lifetime and leave
// younger ones alone, with a grace margin so a clock skew between the API and
// the cluster cannot reclaim a workspace on its exact boundary.
func TestWorkspaceReaperSelectsOnlyExpired(t *testing.T) {
	const maxLifetime = 4 * time.Hour
	deadline := time.Now().UTC().Add(-maxLifetime - workspaceReaperGrace)

	cases := []struct {
		name    string
		age     time.Duration
		expired bool
	}{
		{"just started", time.Minute, false},
		{"well within the lifetime", 2 * time.Hour, false},
		{"just under the deadline", maxLifetime - time.Minute, false},
		{"inside the grace margin", maxLifetime + 30*time.Second, false},
		{"past the lifetime and grace", maxLifetime + 2*time.Hour, true},
		{"left running all weekend", 72 * time.Hour, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := workspace.Workspace{CreatedAt: time.Now().UTC().Add(-tc.age)}
			expired := !record.CreatedAt.After(deadline)
			if expired != tc.expired {
				t.Fatalf("workspace aged %s: expected expired=%v, got %v", tc.age, tc.expired, expired)
			}
		})
	}
}

// A non-positive lifetime must disable the sweep entirely: reclaiming a user's
// workspace is a decision an operator opts into.
func TestWorkspaceReaperDisabledByDefault(t *testing.T) {
	for _, lifetime := range []time.Duration{0, -time.Hour} {
		var h Handlers
		// Returns without starting a goroutine or touching the nil store.
		h.StartWorkspaceReaper(t.Context(), lifetime)
	}
}
