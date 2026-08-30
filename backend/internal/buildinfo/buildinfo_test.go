package buildinfo

import "testing"

// A stamped binary must report its own version whatever the environment says:
// that is the whole point of moving it out of configuration.
func TestStampedVersionCannotBeOverridden(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "0.5.200-ee.1"
	if got := Resolve("0.5.188-dev"); got != "0.5.200-ee.1" {
		t.Fatalf("a stamped version must win over an override, got %q", got)
	}
}

// An unstamped binary may be labelled, so a developer build is not anonymous.
func TestUnstampedFallsBackToOverride(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "dev"
	if got := Resolve("local-experiment"); got != "local-experiment" {
		t.Fatalf("expected the override on an unstamped build, got %q", got)
	}
	if got := Resolve(""); got != "dev" {
		t.Fatalf("expected dev with nothing supplied, got %q", got)
	}
}
