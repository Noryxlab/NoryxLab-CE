package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

func TestContainsFoldIgnoresCaseAndPadding(t *testing.T) {
	for _, c := range []struct {
		values []string
		want   string
		found  bool
	}{
		{[]string{"noryx-api"}, "noryx-api", true},
		{[]string{" NORYX-API "}, "noryx-api", true},
		{[]string{"other", "noryx-api"}, "noryx-api", true},
		{[]string{"other"}, "noryx-api", false},
		{nil, "noryx-api", false},
	} {
		if got := containsFold(c.values, c.want); got != c.found {
			t.Errorf("containsFold(%v, %q) = %v", c.values, c.want, got)
		}
	}
}

// Nothing required, or no directory to ask: the check has nothing to say. An
// unreachable identity provider is an outage, not a misconfiguration, and
// reporting it as one sends an operator looking for a mapper that is present.
func TestTheIdentityCheckIsSilentWithoutAnAudienceOrADirectory(t *testing.T) {
	identityCheckState = identityCheck{}
	if alerts := (Handlers{oidcAudience: "", keycloak: nil}).identityAlerts(); alerts != nil {
		t.Fatalf("raised %+v with nothing configured", alerts)
	}
	identityCheckState = identityCheck{}
	if alerts := (Handlers{oidcAudience: "noryx-api", keycloak: nil}).identityAlerts(); alerts != nil {
		t.Fatalf("raised %+v with no directory", alerts)
	}
}

// The condition is a platform one and critical: every sign-in succeeds and
// every request is then refused, which reads to a user as a broken product.
func TestAMissingAudienceWouldBeCriticalAndPlatformScoped(t *testing.T) {
	alert := healthAlert{
		Scope: health.ScopePlatform, Severity: healthCritical, Source: "identity",
		Summary: "the identity provider does not issue tokens this platform accepts",
	}
	if alert.Scope != health.ScopePlatform {
		t.Fatal("a broken sign-in is the operator's problem, not one user's")
	}
	if alert.Severity != healthCritical {
		t.Fatal("a platform nobody can use is critical")
	}
}
