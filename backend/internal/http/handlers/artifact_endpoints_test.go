package handlers

import (
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
)

func envMap(vars []noryxruntime.EnvVar) map[string]string {
	out := map[string]string{}
	for _, item := range vars {
		out[item.Name] = item.Value
	}
	return out
}

// An installation that declares nothing must behave exactly as before. This is
// the guard on a change that touches every workload: if it injected a default
// index, every existing project would start resolving packages somewhere new
// without anybody asking for it.
func TestNoEndpointDeclaredChangesNothing(t *testing.T) {
	h := Handlers{settings: settingsForTest(t)}
	if got := h.artifactEndpointEnv(); len(got) != 0 {
		t.Fatalf("an undeclared endpoint injected %v", got)
	}
}

// pip refuses a plain-HTTP index unless the host is trusted. An in-cluster
// endpoint has no public name and therefore no certificate, so that is the
// normal case rather than the exception - and the trust is derived from the
// declared URL so it cannot drift from it.
func TestAPlainHTTPIndexAlsoTrustsItsHost(t *testing.T) {
	h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsPythonIndex,
		"http://pulp.pulp.svc.cluster.local/pypi/pypi/simple/")}
	env := envMap(h.artifactEndpointEnv())
	if env["PIP_INDEX_URL"] != "http://pulp.pulp.svc.cluster.local/pypi/pypi/simple/" {
		t.Fatalf("PIP_INDEX_URL = %q", env["PIP_INDEX_URL"])
	}
	if env["PIP_TRUSTED_HOST"] != "pulp.pulp.svc.cluster.local" {
		t.Fatalf("PIP_TRUSTED_HOST = %q", env["PIP_TRUSTED_HOST"])
	}
}

// A port belongs to the trusted host: pip matches on host and port together,
// so trusting the bare name leaves a non-standard port refused.
func TestANonStandardPortIsPartOfTheTrustedHost(t *testing.T) {
	h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsPythonIndex,
		"http://nexus.internal:8081/repository/pypi/simple/")}
	if got := envMap(h.artifactEndpointEnv())["PIP_TRUSTED_HOST"]; got != "nexus.internal:8081" {
		t.Fatalf("PIP_TRUSTED_HOST = %q, want host and port", got)
	}
}

// Over TLS there is nothing to trust, and adding the host anyway would be
// quietly turning off certificate verification for a URL that does not need
// it. A client's Artifactory is the case that matters here.
func TestAnHTTPSIndexTrustsNobody(t *testing.T) {
	h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsPythonIndex,
		"https://artifactory.example.com/artifactory/api/pypi/pypi/simple/")}
	env := envMap(h.artifactEndpointEnv())
	if env["PIP_INDEX_URL"] == "" {
		t.Fatal("the index was not injected")
	}
	if host, present := env["PIP_TRUSTED_HOST"]; present {
		t.Fatalf("an https index trusted %q, weakening verification for nothing", host)
	}
}

// A malformed setting must not become a trusted host. The registry validates
// on write, but a row edited by hand in the database does not pass through it -
// which is exactly the case the egress tests already guard against.
func TestAMalformedIndexTrustsNobody(t *testing.T) {
	for _, value := range []string{"http://", "not a url", "://nowhere"} {
		h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsPythonIndex, value)}
		if host, present := envMap(h.artifactEndpointEnv())["PIP_TRUSTED_HOST"]; present {
			t.Errorf("%q produced the trusted host %q", value, host)
		}
	}
}

// The platform's defaults come first so a caller's own value wins on a
// collision: a workload launched with an explicit PIP_INDEX_URL means somebody
// decided, and a platform default must not silently replace a decision.
func TestTheProjectsOwnVariablesComeAfterTheDefaults(t *testing.T) {
	h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsPythonIndex,
		"http://pulp.pulp.svc.cluster.local/pypi/pypi/simple/")}
	combined := h.workloadBaseEnv([]noryxruntime.EnvVar{{Name: "PIP_INDEX_URL", Value: "http://chosen/simple/"}})
	if len(combined) < 2 {
		t.Fatalf("the project's variables were dropped: %v", combined)
	}
	if last := combined[len(combined)-1]; last.Name != "PIP_INDEX_URL" || last.Value != "http://chosen/simple/" {
		t.Fatalf("the caller's value is not last: %v", combined)
	}
}
