package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The environment screen used to read these over the network. An installation
// with no outbound route - which is what a regulated customer buys this for -
// got "dockerfile fetch failed" for the platform's own environments, whose
// content that same platform was built from.
func TestThePlatformCarriesItsOwnEnvironmentDockerfiles(t *testing.T) {
	for _, path := range []string{
		"environments/noryx-jupyter/Dockerfile",
		"environments/noryx-vscode/Dockerfile",
		"environments/noryx-rstudio/Dockerfile",
	} {
		content, source := embeddedDockerfile(path)
		if content == "" {
			t.Errorf("%s is not embedded: the screen would need the internet to show it", path)
			continue
		}
		if !strings.HasPrefix(content, "FROM ") {
			t.Errorf("%s does not look like a Dockerfile: %.40q", path, content)
		}
		if source != "embedded://"+path {
			t.Errorf("%s reports its source as %q", path, source)
		}
	}

	if content, _ := embeddedDockerfile("environments/somebody-elses/Dockerfile"); content != "" {
		t.Error("a path that is not one of the platform's own must not resolve")
	}
}

// The embedded copy is generated from the real files. A Dockerfile edited
// without regenerating would leave the screen showing last month's file, so
// the two are compared here as well as in CI.
func TestTheEmbeddedDockerfilesMatchTheirSource(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	for path, embedded := range systemDockerfiles {
		onDisk, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if string(onDisk) != embedded {
			t.Errorf("%s has drifted from the embedded copy; run scripts/ops/embed-system-dockerfiles.py", path)
		}
	}
}
