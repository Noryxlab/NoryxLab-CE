package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
)

func TestValidStorageSize(t *testing.T) {
	for _, value := range []string{"1Gi", "10Gi", "250Gi"} {
		if !validStorageSize(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	for _, value := range []string{"", "0Gi", "10", "1Mi", "-1Gi", "abcGi"} {
		if validStorageSize(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestDataServicePodSpec(t *testing.T) {
	for _, definition := range datasource.SystemServiceDefinitions() {
		tier := defaultHardwareTiers()[1]
		spec := dataServicePodSpec("dataservice-test", "dataservice-test-data", "dataservice-test-credentials", "app", "owner", definition, map[string]string{"test": "true"}, "pull-secret", tier)
		if spec.Image != definition.Image || spec.RestartPolicy != "Always" || spec.ReadinessPort != definition.DefaultPort {
			t.Fatalf("unexpected pod spec for %s: %#v", definition.ID, spec)
		}
		if spec.CPULimit != tier.CPULimit || spec.MemLimit != tier.MemoryLimit {
			t.Fatalf("hardware tier not applied for %s: %#v", definition.ID, spec)
		}
		if len(spec.Volumes) != 1 || spec.Volumes[0].ClaimName != "dataservice-test-data" {
			t.Fatalf("missing persistent volume for %s: %#v", definition.ID, spec.Volumes)
		}
		hasPassword := false
		for _, env := range spec.Env {
			if env.SecretName == "dataservice-test-credentials" && env.SecretKey == "password" {
				hasPassword = true
			}
		}
		if !hasPassword {
			t.Fatalf("missing password secret reference for %s", definition.ID)
		}
	}
}
