package k8s

import "testing"

// The log request names a container, and not every job's is called "main": a
// build's is "kaniko". Naming the wrong one is a 400 from the API server,
// which reached the screen as "no logs yet" - so a build that had already
// failed with a clear message looked like a build still starting, for ever.
func TestTheContainerToReadComesFromThePodAndNotFromAnAssumption(t *testing.T) {
	body := []byte(`{"items":[
		{"metadata":{"name":"build-abc-25lxs"},"spec":{"containers":[{"name":"kaniko"}]}},
		{"metadata":{"name":"job-xyz-7f2"},"spec":{"containers":[{"name":"main"},{"name":"sidecar"}]}}
	]}`)

	if got := jobLogContainer(body, "build-abc-25lxs"); got != "kaniko" {
		t.Errorf("a build's logs come from its kaniko container, got %q", got)
	}
	if got := jobLogContainer(body, "job-xyz-7f2"); got != "main" {
		t.Errorf("a job with several containers reads the platform's own, got %q", got)
	}
	// An unknown pod, or a body that does not parse: let the API server pick
	// rather than send it a name that does not exist.
	if got := jobLogContainer(body, "nothing-like-it"); got != "" {
		t.Errorf("an unknown pod should name no container, got %q", got)
	}
	if got := jobLogContainer([]byte("not json"), "build-abc-25lxs"); got != "" {
		t.Errorf("an unreadable listing should name no container, got %q", got)
	}
}
