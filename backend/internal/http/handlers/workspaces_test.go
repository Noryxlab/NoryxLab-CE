package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func TestNodeReachableKubernetesServiceEndpoint(t *testing.T) {
	resolved, err := nodeReachableKubernetesServiceEndpoint(
		"minio.noryx-ce.svc.cluster.local:9000",
		func(host string) ([]string, error) {
			if host != "minio.noryx-ce.svc.cluster.local" {
				t.Fatalf("unexpected host lookup: %s", host)
			}
			return []string{"10.43.249.104"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "10.43.249.104:9000" {
		t.Fatalf("unexpected resolved endpoint: %s", resolved)
	}

	external, err := nodeReachableKubernetesServiceEndpoint(
		"https://cellar-c2.services.clever-cloud.com",
		func(string) ([]string, error) { return nil, errors.New("must not resolve external endpoint") },
	)
	if err != nil || external != "https://cellar-c2.services.clever-cloud.com" {
		t.Fatalf("external endpoint changed: endpoint=%s err=%v", external, err)
	}
}

func TestWorkspaceBootstrapDoesNotSynchronizeDirectDatasetMounts(t *testing.T) {
	script := workspaceBootstrapScript(
		"vscode",
		"workspace-id",
		"token",
		"stef",
		"stef@noryxlab.ai",
		false,
		"/home/noryx/.noryx-profile",
		"/mnt",
		nil,
		2,
	)

	if strings.Contains(script, "from minio import Minio") || strings.Contains(script, "initial_sync") {
		t.Fatal("direct dataset mounts must not trigger local S3 synchronization")
	}
	if !strings.Contains(script, "repos=0 datasets=2") {
		t.Fatal("bootstrap must report the number of direct dataset mounts")
	}
}

func TestWorkspaceBootstrapConfiguresRepositoryGitIdentity(t *testing.T) {
	script := workspaceBootstrapScript(
		"vscode",
		"workspace-id",
		"token",
		"keycloak-user",
		"keycloak@example.org",
		false,
		"/home/noryx/.noryx-profile",
		"/mnt",
		[]workspaceAttachedRepo{{
			Name:           "example",
			URL:            "https://example.org/example.git",
			GitAuthorName:  "Git Author",
			GitAuthorEmail: "git-author@example.org",
		}},
		0,
	)
	for _, expected := range []string{
		"git -C '/repos/example' config user.name 'Git Author'",
		"git -C '/repos/example' config user.email 'git-author@example.org'",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bootstrap does not contain repository identity command %q", expected)
		}
	}
}

func TestWorkspaceAccessURLNeverContainsInternalToken(t *testing.T) {
	for _, kind := range []string{"jupyter", "vscode", "rstudio"} {
		accessURL := workspaceAccessURL(kind, "workspace-id")
		if strings.Contains(accessURL, "token=") {
			t.Fatalf("%s access URL exposes internal workspace token: %s", kind, accessURL)
		}
	}
}

func TestDeriveWorkspaceIDEsFromSystemAndForkedImages(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{name: "jupyter system", values: []string{"harbor.lan/noryx-environments/noryx-jupyter:0.1.0"}, expected: "jupyter"},
		{name: "vscode fork", values: []string{"harbor.lan/project/custom:1", "FROM harbor.lan/noryx-environments/noryx-vscode:0.1.0"}, expected: "vscode"},
		{name: "rstudio fork", values: []string{"harbor.lan/project/custom-r:1", "FROM harbor.lan/noryx-environments/noryx-rstudio:0.1.0"}, expected: "rstudio"},
		{name: "generic job image", values: []string{"harbor.lan/project/batch:1"}, expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(deriveWorkspaceIDEs(tt.values...), ",")
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestRStudioBootstrapUsesWorkspaceRootPath(t *testing.T) {
	script := workspaceBootstrapScript("rstudio", "workspace-id", "", "stef", "stef@noryxlab.ai", false, "/home/noryx/.noryx-profile", "/mnt", nil, 0)
	for _, expected := range []string{"rserver", "--www-root-path=/workspaces/workspace-id", "--server-user=noryx"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("RStudio bootstrap missing %s", expected)
		}
	}
}

func TestWorkspaceBootstrapConfiguresPersistentGitIdentity(t *testing.T) {
	script := workspaceBootstrapScript("vscode", "workspace-id", "", "stef", "stef@noryxlab.ai", false, "/home/noryx/.noryx-profile", "/mnt", nil, 0)
	for _, expected := range []string{
		"git config --file '/home/noryx/.noryx-profile/gitconfig' user.name 'stef'",
		"git config --file '/home/noryx/.noryx-profile/gitconfig' user.email 'stef@noryxlab.ai'",
		"ln -sfn '/home/noryx/.noryx-profile/gitconfig' /home/noryx/.gitconfig",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("workspace bootstrap missing persistent Git identity command: %s", expected)
		}
	}
}

func TestUserSecretEnvName(t *testing.T) {
	if got := userSecretEnvName("key-vastai-stephane"); got != "NORYX_SECRET_KEY_VASTAI_STEPHANE" {
		t.Fatalf("unexpected secret env name: %s", got)
	}
}

func TestSystemEnvironmentDefinitionIncludesDockerfile(t *testing.T) {
	definition, ok := getSystemEnvironmentDefinition("system-vscode")
	if !ok || definition.DockerfilePath != "environments/noryx-vscode/Dockerfile" || definition.GitRef == "" {
		t.Fatalf("invalid system VSCode environment definition: %#v", definition)
	}
}

func TestWorkspaceProxyRejectsSharedTokenWithoutSSO(t *testing.T) {
	workspaces := memory.NewWorkspaceStore()
	record := workspace.New(
		"jupyter", "project-id", "private-workspace", "image", "pod", "service",
		"500m", "512Mi", "/workspaces/workspace-id/lab?reset", "legacy-shared-token",
	)
	if err := workspaces.Create(record); err != nil {
		t.Fatal(err)
	}

	h := Handlers{workspaceStore: workspaces}
	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+record.ID+"/lab?token=legacy-shared-token", nil)
	request.SetPathValue("workspaceID", record.ID)
	request.SetPathValue("path", "lab")
	response := httptest.NewRecorder()

	h.ProxyWorkspace(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("shared workspace URL must require SSO, got HTTP %d", response.Code)
	}
}

func TestWorkspaceProxyTargetPath(t *testing.T) {
	tests := []struct {
		kind     string
		rest     string
		expected string
	}{
		{kind: "jupyter", rest: "lab", expected: "/workspaces/workspace-id/lab"},
		{kind: "vscode", rest: "", expected: "/workspaces/workspace-id"},
		{kind: "rstudio", rest: "", expected: "/"},
		{kind: "rstudio", rest: "auth-sign-in", expected: "/auth-sign-in"},
	}
	for _, test := range tests {
		if actual := workspaceProxyTargetPath(test.kind, "workspace-id", test.rest); actual != test.expected {
			t.Fatalf("%s proxy target: got %q, want %q", test.kind, actual, test.expected)
		}
	}
}
