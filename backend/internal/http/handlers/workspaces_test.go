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
			return []string{"10.0.0.10"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "10.0.0.10:9000" {
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
		"admin@example.org",
		false,
		"/home/noryx/.noryx-profile",
		"/mnt",
		nil,
		2,
		"",
		false,
		0,
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
		"",
		false,
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

func TestRepositoryBootstrapUsesSecretEnvironmentWithoutEmbeddingToken(t *testing.T) {
	script := strings.Join(repositoryBootstrapLines(workspaceAttachedRepo{
		Name:        "example",
		URL:         "https://example.org/example.git",
		AuthEnvName: "NORYX_SECRET_GITHUB_TOKEN",
	}, "/repos/example"), "\n")

	for _, expected := range []string{"GIT_ASKPASS=", "$NORYX_SECRET_GITHUB_TOKEN", "https://example.org/example.git", "noryx-git-credential-example", "config --replace-all credential.helper ''", "config --add credential.helper", "config credential.interactive never"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bootstrap does not contain %q", expected)
		}
	}
	if strings.Contains(script, "oauth2:") {
		t.Fatal("bootstrap must not embed authenticated repository URLs")
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
		{name: "jupyter system", values: []string{"harbor.example.local/noryx-environments/noryx-jupyter:0.1.0"}, expected: "jupyter"},
		{name: "vscode fork", values: []string{"harbor.example.local/project/custom:1", "FROM harbor.example.local/noryx-environments/noryx-vscode:0.1.2"}, expected: "vscode"},
		{name: "rstudio fork", values: []string{"harbor.example.local/project/custom-r:1", "FROM harbor.example.local/noryx-environments/noryx-rstudio:0.1.0"}, expected: "rstudio"},
		{name: "generic job image", values: []string{"harbor.example.local/project/batch:1"}, expected: ""},
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
	script := workspaceBootstrapScript("rstudio", "workspace-id", "", "stef", "admin@example.org", false, "/home/noryx/.noryx-profile", "/mnt", nil, 0, "", false, 0)
	for _, expected := range []string{"rserver", "--www-root-path=/workspaces/workspace-id", "--server-user=noryx"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("RStudio bootstrap missing %s", expected)
		}
	}
}

func TestWorkspaceBootstrapConfiguresPersistentGitIdentity(t *testing.T) {
	script := workspaceBootstrapScript("vscode", "workspace-id", "", "stef", "admin@example.org", false, "/home/noryx/.noryx-profile", "/mnt", nil, 0, "", false, 0)
	for _, expected := range []string{
		"git config --file '/home/noryx/.noryx-profile/gitconfig' user.name 'stef'",
		"git config --file '/home/noryx/.noryx-profile/gitconfig' user.email 'admin@example.org'",
		"ln -sfn '/home/noryx/.noryx-profile/gitconfig' /home/noryx/.gitconfig",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("workspace bootstrap missing persistent Git identity command: %s", expected)
		}
	}
}

func TestWorkspaceBootstrapConfiguresContinueAssistant(t *testing.T) {
	config := continueDeveloperAssistantConfig("https://datalab.example.org/", "developer-token", "Premyom")
	script := workspaceBootstrapScript("vscode", "workspace-id", "", "stef", "admin@example.org", false, "/home/noryx/.noryx-profile", "/mnt", nil, 0, config, false, 0)
	for _, expected := range []string{
		"/opt/noryx-vscode/extensions",
		"chmod 755 /mnt/lost+found",
		"/home/noryx/.continue/config.yaml",
		"--user-data-dir '/home/noryx/.noryx-profile/vscode/data'",
		"--extensions-dir '/home/noryx/.noryx-profile/vscode/extensions'",
		"apiBase: 'https://datalab.example.org/api/v1/assistant/developer/v1'",
		"apiKey: 'developer-token'",
		"useResponsesApi: false",
		"capabilities:",
		"tool_use",
		"provider: code",
		"/repos",
		"Git credentials are provisioned by Premyom",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("workspace bootstrap missing Continue assistant config: %s", expected)
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

// What a person reads in the IDE assistant panel follows the installation's
// name. It said "Noryx Assistant" on a platform branded Premyom, which is the
// same branding leak as the greeting, one layer down.
func TestContinueConfigCarriesTheProductName(t *testing.T) {
	config := continueDeveloperAssistantConfig("https://datalab.example.org", "token", "Premyom")
	for _, expected := range []string{"name: Premyom Workspace", "  - name: Premyom Assistant"} {
		if !strings.Contains(config, expected) {
			t.Fatalf("Continue config missing %q:\n%s", expected, config)
		}
	}
	if strings.Contains(config, "Noryx Assistant") {
		t.Fatal("Continue config still names Noryx on a Premyom install")
	}
}

// An unnamed installation keeps the default rather than an empty name.
func TestContinueConfigFallsBackToNoryx(t *testing.T) {
	if !strings.Contains(continueDeveloperAssistantConfig("https://x", "t", " "), "name: Noryx Workspace") {
		t.Fatal("an unnamed installation must fall back to Noryx")
	}
}

// The address the IDE is pointed at has to be one a pod can reach. Pointed at
// the platform's public name, Continue had no route at all: from inside the
// cluster that name times out on its own NAT address, and the private service
// address is excluded by the workspace egress policy.
func TestContinueConfigUsesAReachableEndpoint(t *testing.T) {
	config := continueDeveloperAssistantConfig("http://noryx-backend.noryx.svc.cluster.local:8080", "token", "Premyom")
	if !strings.Contains(config, "apiBase: 'http://noryx-backend.noryx.svc.cluster.local:8080/api/v1/assistant/developer/v1'") {
		t.Fatalf("Continue config does not point at the in-cluster endpoint:\n%s", config)
	}
}

// A rebuilt record must say what the pod is, not what the platform would have
// given it.
//
// The reconciler wrote the platform defaults for size and the literal word
// "running" for status. A workspace launched at 1 CPU and 4 GiB was displayed
// as 500m and 512Mi - the interface contradicting a number its owner had
// chosen, on the screen they would consult to decide whether to choose a
// bigger one. And one killed for memory at eleven was still listed as running
// at three.
func TestARebuiltRecordCarriesThePodsOwnSizeAndPhase(t *testing.T) {
	if got := firstNonEmptyString("1", "500m"); got != "1" {
		t.Fatalf("cpu = %q, want the pod's own limit", got)
	}
	if got := firstNonEmptyString("4Gi", "512Mi"); got != "4Gi" {
		t.Fatalf("memory = %q, want the pod's own limit", got)
	}
	// A pod that declares no limit falls back, rather than showing nothing.
	if got := firstNonEmptyString("", "512Mi"); got != "512Mi" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestAPodPhaseBecomesTheWorkspaceStatus(t *testing.T) {
	for phase, want := range map[string]string{
		"failed":    "failed",
		"Failed":    "failed",
		"succeeded": "stopped",
		"pending":   "launching",
		"running":   "running",
	} {
		if got := workspaceStatusFromPhase(phase); got != want {
			t.Fatalf("phase %q became %q, want %q", phase, got, want)
		}
	}
	// A phase nobody has seen keeps the optimistic answer: inventing "failed"
	// on a reconciliation sweep would be a worse lie than the one this
	// replaces.
	if got := workspaceStatusFromPhase("quelque-chose-de-nouveau"); got != "running" {
		t.Fatalf("unknown phase became %q", got)
	}
	if got := workspaceStatusFromPhase(""); got != "running" {
		t.Fatalf("empty phase became %q", got)
	}
}

// The assistant keeps its codebase index in the profile volume so it survives
// a workspace being stopped, and the profile volume mounts under the
// container's home. Listing home as a workspace folder therefore told the
// assistant to index its own index - a directory that grows because it is
// being indexed. The same walk opened the root of an ext4 volume and logged
// "EACCES: scandir .../lost+found" on every launch.
func TestTheWorkspaceDoesNotOfferTheContainersHomeAsAFolder(t *testing.T) {
	script := workspaceBootstrapScript("vscode", "workspace-id", "", "stef", "admin@example.org", false, "/home/noryx/.noryx-profile", "/mnt", nil, 0, "", false, 0)

	if !strings.Contains(script, `{ "path": "/mnt" }`) {
		t.Fatal("the project volume must still be a workspace folder")
	}
	for _, folder := range []string{`"path": "/home"`, `"path": "/home/noryx"`} {
		if strings.Contains(script, folder) {
			t.Errorf("home is a workspace folder again: %s", folder)
		}
	}
	// The profile volume must not be reachable as a root by any other spelling.
	if strings.Contains(script, `"path": "`+defaultWorkspaceProfileDir) {
		t.Error("the profile volume is a workspace folder")
	}
}

// The assistant's tools run as the person, with their filesystem. On
// 2026-09-18 one was asked to list /datasets and returned the contents of an
// HDS dataset, which then travelled to the model endpoint - out of the site,
// through a gateway, to a rented GPU. Nobody decided that; it followed from
// the assistant existing in a workspace that had the data mounted.
//
// A rule in the prompt is not a control: the model may ignore it and the tools
// never read it. Withholding the endpoint and the token is.
func TestRegulatedDataIsRecognisedWhateverTheSpelling(t *testing.T) {
	ordinary := []workspaceAttachedDataset{{Name: "testdataset", Classification: "non-hds"}}
	if anyRegulatedDataset(ordinary) {
		t.Error("an ordinary dataset must not withhold the assistant")
	}
	if anyRegulatedDataset(nil) {
		t.Error("a workspace with no dataset must not withhold the assistant")
	}

	// The label is written by whoever declared the dataset, so case and
	// padding are what a person typed rather than a guarantee.
	for _, label := range []string{"hds", "HDS", "Hds", " hds "} {
		mixed := []workspaceAttachedDataset{
			{Name: "testdataset", Classification: "non-hds"},
			{Name: "hds-for", Classification: label},
		}
		if !anyRegulatedDataset(mixed) {
			t.Errorf("classification %q was not recognised as regulated", label)
		}
	}
}
