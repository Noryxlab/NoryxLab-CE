package projectvar

import "testing"

func TestANameHasToBeOneTheShellCanExport(t *testing.T) {
	for _, name := range []string{"MLFLOW_TRACKING_URI", "bucket", "_internal", "S3_ENDPOINT2"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("%s should be accepted: %v", name, err)
		}
	}
	for _, name := range []string{"", "2FAST", "with-dash", "with space", "accentué"} {
		if ValidateName(name) == nil {
			t.Errorf("%q should be refused: nothing could export it", name)
		}
	}
}

// A project that could set these would be rewriting the environment its own
// workload depends on, and the failure would look like a platform bug.
func TestThePlatformKeepsItsOwnNames(t *testing.T) {
	for _, name := range []string{"NORYX_SECRET_TOKEN", "noryx_anything", "PATH", "home", "KUBERNETES_SERVICE_HOST"} {
		if ValidateName(name) == nil {
			t.Errorf("%q should be refused: it belongs to the platform or the shell", name)
		}
	}
}
