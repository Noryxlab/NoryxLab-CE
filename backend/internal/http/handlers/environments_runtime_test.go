package handlers

import "testing"

// The runtime environment must be offered where an environment is chosen, and
// never where a workspace IDE is.
//
// The empty IDE list is the whole semantic: a workspace launched on this image
// would start and have nothing to show, because it carries no editor. Before
// it existed the smallest system image with Python in it was noryx-rstudio at
// 2.58 GB, which is what the accounting service ran on - and which also meant
// its base changed whenever a VS Code extension moved and the nightly rebuild
// picked it up.
func TestTheRuntimeEnvironmentOffersNoIDE(t *testing.T) {
	definition, declared := systemEnvironmentDefinitions["system-runtime"]
	if !declared {
		t.Fatal("system-runtime is not in the catalogue, so nothing can be launched on it")
	}
	if len(definition.WorkspaceIDEs) != 0 {
		t.Errorf("the runtime environment offers IDEs %v; a workspace on it would show nothing",
			definition.WorkspaceIDEs)
	}
	if definition.DockerfilePath != "environments/noryx-runtime/Dockerfile" {
		t.Errorf("DockerfilePath = %q, so a rebuild would build something else",
			definition.DockerfilePath)
	}
}

// Every other system environment must keep an IDE: this change adds a kind
// rather than redefining the existing ones.
func TestTheIDEEnvironmentsStillDeclareTheirs(t *testing.T) {
	for key, want := range map[string]string{
		"system-jupyter": "jupyter",
		"system-vscode":  "vscode",
		"system-rstudio": "rstudio",
	} {
		definition := systemEnvironmentDefinitions[key]
		if len(definition.WorkspaceIDEs) != 1 || definition.WorkspaceIDEs[0] != want {
			t.Errorf("%s declares %v, want exactly [%s]", key, definition.WorkspaceIDEs, want)
		}
	}
}
