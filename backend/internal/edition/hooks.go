package edition

import (
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

const (
	FeatureCustomRBACMatrix = "custom_rbac_matrix"
	FeatureAdvancedAudit    = "advanced_audit"
	FeaturePolicyEngine     = "policy_engine"
	FeatureHDSDatasets      = "hds_datasets"
	FeatureControlledEgress = "controlled_egress"
)

type RBACProvider interface {
	// IsGlobalAdmin can override CE default global-admin resolution.
	IsGlobalAdmin(identity auth.Identity, fallback func(auth.Identity) bool) bool
	// CanAccessAdminModule can override access to admin modules (users/modules/workloads).
	CanAccessAdminModule(identity auth.Identity, module string, fallback bool) bool
	// CanAccessProjectAction can implement matrix-based project permissions (EE).
	CanAccessProjectAction(identity auth.Identity, projectID string, role access.Role, action string, fallback bool) bool
}

type FeatureGate interface {
	Enabled(feature string) bool
}

type AuditSink interface {
	Record(event string, fields map[string]string)
}

type Hooks struct {
	RBAC    RBACProvider
	Feature FeatureGate
	Audit   AuditSink
}

type defaultRBACProvider struct{}

func (defaultRBACProvider) IsGlobalAdmin(identity auth.Identity, fallback func(auth.Identity) bool) bool {
	return fallback(identity)
}

func (defaultRBACProvider) CanAccessAdminModule(_ auth.Identity, _ string, fallback bool) bool {
	return fallback
}

func (defaultRBACProvider) CanAccessProjectAction(_ auth.Identity, _ string, _ access.Role, _ string, fallback bool) bool {
	return fallback
}

type defaultFeatureGate struct{}

func (defaultFeatureGate) Enabled(string) bool { return false }

type staticFeatureGate map[string]bool

func (g staticFeatureGate) Enabled(feature string) bool {
	return g[strings.ToLower(strings.TrimSpace(feature))]
}

func FeatureGateFromCSV(raw string) FeatureGate {
	features := staticFeatureGate{}
	for _, feature := range strings.Split(raw, ",") {
		if normalized := strings.ToLower(strings.TrimSpace(feature)); normalized != "" {
			features[normalized] = true
		}
	}
	return features
}

type noopAuditSink struct{}

func (noopAuditSink) Record(string, map[string]string) {}

func DefaultHooks() Hooks {
	return Hooks{
		RBAC:    defaultRBACProvider{},
		Feature: defaultFeatureGate{},
		Audit:   noopAuditSink{},
	}
}
