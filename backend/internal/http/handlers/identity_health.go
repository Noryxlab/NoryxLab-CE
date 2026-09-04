package handlers

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

// Whether the identity provider will issue tokens this platform accepts.
//
// The backend requires an audience, and the realm has to be configured to put
// it in the token. Nothing connected the two: a realm rebuilt, or a customer
// installed without that undocumented step, produced a platform that
// authenticated a user and then refused every request they made with an opaque
// "invalid bearer token" - visible to nobody as a configuration problem.
//
// The platform has everything needed to notice: it knows the audience it
// requires and it holds administrator credentials on the realm. It should
// therefore say so itself rather than leave an operator to guess, which is the
// same argument as every other check in the health report.
const identityCheckInterval = 10 * time.Minute

type identityCheck struct {
	mu        sync.Mutex
	checkedAt time.Time
	alerts    []healthAlert
}

var identityCheckState identityCheck

func (h Handlers) identityAlerts() []healthAlert {
	audience := strings.TrimSpace(h.oidcAudience)
	if audience == "" || h.keycloak == nil {
		// No audience required, or no directory to ask: nothing to say. An
		// unreachable directory is not a configuration error and must not be
		// reported as one.
		return nil
	}

	// Cached: the health report is computed on every request to the screen, and
	// asking Keycloak each time would turn an operator refreshing a page into a
	// load generator against the identity provider.
	identityCheckState.mu.Lock()
	defer identityCheckState.mu.Unlock()
	if time.Since(identityCheckState.checkedAt) < identityCheckInterval {
		return identityCheckState.alerts
	}

	alerts := []healthAlert(nil)
	audiences, err := h.keycloak.ClientAudiences(h.oidcFrontendClientID)
	switch {
	case err != nil:
		// Reaching the directory failed. Silent on purpose: an outage here
		// would otherwise be reported as a misconfiguration, and an operator
		// would go looking for a mapper that is present.
		log.Printf("identity check: cannot read the audiences of client %q: %v", h.oidcFrontendClientID, err)
	case !containsFold(audiences, audience):
		alerts = []healthAlert{{
			Scope:    health.ScopePlatform,
			Severity: healthCritical,
			Source:   "identity",
			Summary:  "the identity provider does not issue tokens this platform accepts",
			Detail: "client " + h.oidcFrontendClientID + " adds no audience " + audience +
				"; every sign-in will succeed and every request will then be refused",
			Action: "identity",
		}}
	}

	identityCheckState.checkedAt = time.Now()
	identityCheckState.alerts = alerts
	return alerts
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}
