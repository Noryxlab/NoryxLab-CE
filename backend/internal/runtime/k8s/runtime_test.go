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

func TestKanikoBuildArgsUsesLocalContextForInlineDockerfile(t *testing.T) {
	args, dockerfile := kanikoBuildArgs(noryxruntime.BuildSpec{
		ContextGitURL:     "https://github.com/Noryxlab/NoryxLab-CE.git",
		GitRef:            "main",
		DockerfilePath:    "environments/noryx-vscode/Dockerfile",
		DockerfileContent: "FROM example/base:latest",
		ContextPath:       ".",
		DestinationImage:  "harbor.lan/noryx-environments/custom:0.1.0",
	})

	got := strings.Join(args, " ")
	if !strings.Contains(got, "--context=dir:///workspace") {
		t.Fatalf("inline Dockerfile must use local context: %s", got)
	}
	if strings.Contains(got, "--context-sub-path=") || strings.Contains(got, "github.com") {
		t.Fatalf("inline Dockerfile must not fetch a Git context: %s", got)
	}
	if dockerfile != "/workspace/Dockerfile" {
		t.Fatalf("unexpected inline Dockerfile path: %s", dockerfile)
	}
}

func TestKanikoBuildArgsKeepsGitContextForRepositoryBuild(t *testing.T) {
	args, dockerfile := kanikoBuildArgs(noryxruntime.BuildSpec{
		ContextGitURL:    "https://github.com/Noryxlab/NoryxLab-CE.git",
		GitRef:           "main",
		DockerfilePath:   "environments/noryx-vscode/Dockerfile",
		ContextPath:      ".",
		DestinationImage: "harbor.lan/noryx-environments/custom:0.1.0",
	})

	got := strings.Join(args, " ")
	if !strings.Contains(got, "--context=https://github.com/Noryxlab/NoryxLab-CE.git#refs/heads/main") ||
		!strings.Contains(got, "--context-sub-path=.") {
		t.Fatalf("repository build must keep its Git context: %s", got)
	}
	if dockerfile != "environments/noryx-vscode/Dockerfile" {
		t.Fatalf("unexpected repository Dockerfile path: %s", dockerfile)
	}
}
