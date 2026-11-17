package handlers

import (
	"strings"
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

// conda needs no file and no image change: every .condarc key has an
// environment twin. One alias redirects every channel name, which is what the
// images need - they ship Miniforge with no explicit channel list.
func TestTheCondaAliasIsInjectedOnItsOwn(t *testing.T) {
	h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsCondaChannelAlias,
		"http://conda-cache.conda-cache.svc.cluster.local")}
	env := envMap(h.artifactEndpointEnv())
	if env["CONDA_CHANNEL_ALIAS"] != "http://conda-cache.conda-cache.svc.cluster.local" {
		t.Fatalf("CONDA_CHANNEL_ALIAS = %q", env["CONDA_CHANNEL_ALIAS"])
	}
	// A conda alias must not drag pip's variables in with it: an installation
	// may well mirror one ecosystem and not the other.
	if _, present := env["PIP_INDEX_URL"]; present {
		t.Error("an undeclared python index was injected anyway")
	}
	if _, present := env["PIP_TRUSTED_HOST"]; present {
		t.Error("a trusted host appeared with no index to trust")
	}
}

// apt is the leg with no environment form, so the variable exists only for the
// bootstrap to turn into a sources.list.
func TestTheDebMirrorIsCarriedForTheBootstrap(t *testing.T) {
	h := Handlers{settings: settingsWithRawValue(t, settings.KeyArtifactsDebMirror,
		"http://pulp.pulp.svc.cluster.local/pulp/content/deb")}
	if got := envMap(h.artifactEndpointEnv())["NORYX_DEB_MIRROR"]; got != "http://pulp.pulp.svc.cluster.local/pulp/content/deb" {
		t.Fatalf("NORYX_DEB_MIRROR = %q", got)
	}
}

// The bootstrap must not touch /etc/apt when nothing is declared, and must say
// what it did when it does. The guard matters because setting the image's own
// sources aside is destructive for a workload that could have reached the
// archive by itself.
func TestTheAptBootstrapIsGuardedAndSpeaks(t *testing.T) {
	script := strings.Join(debMirrorBootstrapLines(), "\n")
	if !strings.Contains(script, `[ -n "${NORYX_DEB_MIRROR:-}" ]`) {
		t.Error("the apt rewrite is not guarded by the variable being set")
	}
	if !strings.Contains(script, "VERSION_CODENAME") {
		t.Error("the release is not read from the image, so the platform would have to know it")
	}
	if !strings.Contains(script, "sources are set aside") {
		t.Error("the script does not say it replaced the image's sources")
	}
	if !strings.Contains(script, "not writable by") {
		t.Error("a failure to write would pass in silence")
	}
}
