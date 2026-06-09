package k8s

import (
	"encoding/json"
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

func TestKubernetesEnvVarsUsesSecretKeyRefWithoutValue(t *testing.T) {
	items := kubernetesEnvVars([]noryxruntime.EnvVar{{
		Name:       "NORYX_SECRET_API_KEY",
		SecretName: "workload-user-secrets",
		SecretKey:  "NORYX_SECRET_API_KEY",
	}})
	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	got := string(payload)
	if !strings.Contains(got, `"secretKeyRef"`) || strings.Contains(got, `"value"`) {
		t.Fatalf("secret env must use secretKeyRef without clear value: %s", got)
	}
}
