package handlers

import (
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/egress"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
)

// workloadEgressProfile names the network profile a workload is launched
// under, which decides whether it reaches the public internet at all.
//
// ADR-023 defined the profiles and ADR-023's own rollout note called the open
// policy "explicitly transitional". It stayed open for every workload,
// including a workspace mounting regulated data: the pod carries a shell,
// curl and sudo, and the namespace allowed every public address. Disabling the
// download button in an editor against that is a sign on an open door.
//
// The profile is derived from the data the project holds rather than from the
// approval workflow, and that is deliberate. A NetworkPolicy filters
// addresses and ports; it cannot filter a domain name, so the two profiles
// that name destinations - package-access and approved-egress - cannot be
// applied at this layer at all. They stay a record until the gateway ADR-023
// describes exists, and are treated as open here rather than pretended to be
// narrow. What can be applied exactly is "closed", and that is the one the
// regulated case needs.
//
// An empty answer is impossible on purpose: a workload whose profile could
// not be decided is labelled, and a label is what the policy selects on, so
// silence cannot become permission.
func (h Handlers) workloadEgressProfile(projectID string) string {
	// A workload with no project is the platform's own helper - the project
	// file browser - and it has no business on the internet.
	if strings.TrimSpace(projectID) == "" {
		return egress.ProfileIsolated
	}
	switch h.egressEnforcement() {
	case settings.EgressEnforcementOff:
		return egress.ProfileUnrestricted
	case settings.EgressEnforcementStrict:
		if h.projectHoldsRegulatedDataNamed(projectID) != "" {
			return egress.ProfileHDSRegulated
		}
		return egress.ProfileIsolated
	default:
		// "regulated": the regulated project is closed and nothing else
		// changes. This is the default because it is the rule nobody argues
		// with - a bucket of patient images has no reason to reach an address
		// somebody typed - while closing every workload breaks a pip install
		// that was legitimate the day before.
		if h.projectHoldsRegulatedDataNamed(projectID) != "" {
			return egress.ProfileHDSRegulated
		}
		return egress.ProfileUnrestricted
	}
}

func (h Handlers) egressEnforcement() string {
	if h.settings == nil {
		return settings.EgressEnforcementRegulated
	}
	value := strings.TrimSpace(h.settings.String(settings.KeyEgressEnforcement))
	switch value {
	case settings.EgressEnforcementOff, settings.EgressEnforcementRegulated, settings.EgressEnforcementStrict:
		return value
	default:
		return settings.EgressEnforcementRegulated
	}
}

// egressProfileClosed reports whether a profile denies direct internet
// access, which is what a screen says to the person launching the workload.
func egressProfileClosed(profile string) bool {
	switch strings.TrimSpace(profile) {
	case egress.ProfileIsolated, egress.ProfileInternalOnly, egress.ProfileHDSRegulated:
		return true
	default:
		return false
	}
}

// withEgressProfile puts the profile on the labels a workload is created
// with, which is what the network policies select on.
//
// The label, not an absence: a policy that admitted pods matching no profile
// would turn every call site somebody forgets into an open door, and the
// runtime defaults an unnamed profile to "unrestricted" so the gap is
// readable on the pod itself rather than invisible.
func withEgressProfile(labels map[string]string, profile string) map[string]string {
	out := make(map[string]string, len(labels)+1)
	for key, value := range labels {
		out[key] = value
	}
	if trimmed := strings.TrimSpace(profile); trimmed != "" {
		out["noryx.io/egress-profile"] = trimmed
	}
	return out
}
