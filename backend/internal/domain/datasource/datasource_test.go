package datasource

import (
	"strings"
	"testing"
)

func TestNewDatasourceIsExternal(t *testing.T) {
	item := New("stef", "risk", "postgres", "db.internal", "risk", "user", "secret", "require", 5432)
	if item.Source != "external" {
		t.Fatalf("expected external datasource, got %q", item.Source)
	}
}

func TestSystemServiceDefinitionsUseImmutableHarborImages(t *testing.T) {
	items := SystemServiceDefinitions()
	if len(items) != 3 {
		t.Fatalf("expected 3 system definitions, got %d", len(items))
	}
	for _, item := range items {
		if !item.System {
			t.Fatalf("%s must be a system definition", item.ID)
		}
		if item.Image == "" || item.Dockerfile == "" {
			t.Fatalf("%s must expose image and dockerfile", item.ID)
		}
		if !strings.HasPrefix(item.Image, "harbor.lan/noryx-dataservices/") || !strings.Contains(item.Image, "@sha256:") {
			t.Fatalf("%s image must be an immutable Harbor reference: %s", item.ID, item.Image)
		}
	}
}
