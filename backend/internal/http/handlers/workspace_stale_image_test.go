package handlers

import "testing"

// A stale tag of the platform's own image is the same environment, one
// revision behind - not a different one.
//
// The launch form is built from a list the browser fetched earlier. When an
// administrator moves a kind to a new image, every already-loaded list keeps
// offering the previous tag and every launch from it was refused, with a
// message the person reading it could not act on. That blocked a user on
// 2026-09-24 who had changed nothing.
func TestStaleRevisionSharesTheRepository(t *testing.T) {
	configured := "harbor.emse.local/noryx-ee/noryx-slicer:0.2.0"
	for _, stale := range []string{
		"harbor.emse.local/noryx-ee/noryx-slicer:0.3.0-kasm",
		"harbor.emse.local/noryx-ee/noryx-slicer:0.1.0",
		"harbor.emse.local/noryx-ee/noryx-slicer@sha256:" +
			"0000000000000000000000000000000000000000000000000000000000000000",
	} {
		if imageRepository(stale) != imageRepository(configured) {
			t.Fatalf("%q was not recognised as the same repository as %q", stale, configured)
		}
	}
}

// And anything else stays refused.
//
// The substitution runs the platform's own image rather than the requested
// tag, so it cannot be used to reach another one - but it must not quietly
// turn a request for a different environment into this one either, because
// somebody launching an editor and getting an imaging workspace learns
// nothing about why.
func TestAnotherImageIsNotAStaleRevision(t *testing.T) {
	configured := "harbor.emse.local/noryx-ee/noryx-slicer:0.2.0"
	for _, other := range []string{
		"harbor.emse.local/noryx-ee/noryx-vscode:0.1.2",
		"harbor.emse.local/noryx-ce/noryx-slicer:0.2.0",
		"docker.io/library/ubuntu:24.04",
		"harbor.emse.local/noryx-ee/noryx-slicer-fork:0.2.0",
	} {
		if imageRepository(other) == imageRepository(configured) {
			t.Fatalf("%q was wrongly treated as a revision of %q", other, configured)
		}
	}
}
