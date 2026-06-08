package edition

import "testing"

func TestFeatureGateFromCSV(t *testing.T) {
	gate := FeatureGateFromCSV("hds_datasets, advanced_audit")
	if !gate.Enabled(FeatureHDSDatasets) || !gate.Enabled(FeatureAdvancedAudit) {
		t.Fatal("expected configured Enterprise features to be enabled")
	}
	if gate.Enabled(FeaturePolicyEngine) {
		t.Fatal("expected unspecified feature to remain disabled")
	}
}
