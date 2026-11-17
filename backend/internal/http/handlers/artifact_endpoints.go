package handlers

import (
	"net"
	"net/url"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
)

// Where a workload installs packages from.
//
// ADR-023 takes the public internet away from a regulated workload. ADR-054
// says what replaces it, and insists on the order: the index is configured
// first, the profile is closed second. The reason is a failure mode rather
// than tidiness - a `pip install` against an unreachable index does not
// refuse, it waits and then times out, which sends the reader to the wrong
// question. This project has paid for that shape twice already, in a gateway
// refusal that blamed the opening hours and in tunnel forwards that listened
// on behalf of deleted machines.
//
// So this exists before the closing does, and an installation that configures
// nothing keeps pip's own default: no behaviour changes until an endpoint is
// declared.

// artifactEndpointEnv is the package configuration every workload inherits.
//
// Injected as environment rather than written into a config file, because the
// environment reaches every tool in the pod without the image having to
// cooperate, and because a file would have to be merged with whatever the user
// already has in ~/.config/pip/pip.conf.
func (h Handlers) artifactEndpointEnv() []noryxruntime.EnvVar {
	index := h.artifactPythonIndex()
	if index == "" {
		return nil
	}
	env := []noryxruntime.EnvVar{{Name: "PIP_INDEX_URL", Value: index}}
	if host := plainHTTPIndexHost(index); host != "" {
		// pip refuses an index served over plain HTTP unless the host is
		// trusted explicitly. An in-cluster endpoint is the normal case for
		// that - it has no public name and therefore no certificate - so the
		// trust is derived from the declared URL instead of being a second
		// setting somebody can leave inconsistent with the first.
		//
		// Derived only for http. Adding it for an https endpoint would be
		// silently weakening certificate verification on a URL that does not
		// need it.
		env = append(env, noryxruntime.EnvVar{Name: "PIP_TRUSTED_HOST", Value: host})
	}
	return env
}

func (h Handlers) artifactPythonIndex() string {
	if h.settings == nil {
		return ""
	}
	return strings.TrimSpace(h.settings.String(settings.KeyArtifactsPythonIndex))
}

// plainHTTPIndexHost returns the host to trust when, and only when, the index
// is served without TLS. An unparseable value yields nothing rather than a
// guess: a malformed setting must not turn into a trusted host.
func plainHTTPIndexHost(index string) string {
	parsed, err := url.Parse(index)
	if err != nil || !strings.EqualFold(parsed.Scheme, "http") {
		return ""
	}
	host := parsed.Hostname()
	if host == "" {
		return ""
	}
	if port := parsed.Port(); port != "" {
		return net.JoinHostPort(host, port)
	}
	return host
}

// workloadBaseEnv puts the declared endpoints before a project's own
// datasource variables.
//
// A helper rather than two `append` calls at each call site, because there are
// four places that build a workload's environment and the one that gets
// forgotten is the one that breaks after the network closes - silently, and
// only for whoever launched that kind of workload.
func (h Handlers) workloadBaseEnv(projectEnv []noryxruntime.EnvVar) []noryxruntime.EnvVar {
	base := h.artifactEndpointEnv()
	if len(base) == 0 {
		return projectEnv
	}
	return append(base, projectEnv...)
}
