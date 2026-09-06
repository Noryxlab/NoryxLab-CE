package http

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The API document and the API.
//
// The document described 40 paths while the router served 181, and nothing
// said so. A client reading it saw a fifth of the surface and had no way to
// know which fifth - which teaches people to read the source instead, and then
// the document stops being maintained at all because nobody uses it.
//
// This reads the routes out of this package's own source, so an Enterprise
// build checks its extra routes too: the overlay lands them right here.
func TestEveryRegisteredRouteIsInTheAPIDocument(t *testing.T) {
	registration := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|DELETE|PATCH) ([^"]+)"`)

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string][]string{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		body, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range registration.FindAllStringSubmatch(string(body), -1) {
			path := strings.ReplaceAll(match[2], "...", "")
			registered[path] = append(registered[path], strings.ToLower(match[1]))
		}
	}
	if len(registered) == 0 {
		t.Fatal("no route found in this package: the test has stopped checking anything")
	}

	raw, err := os.ReadFile(filepath.Join("static", "openapi.yaml"))
	if err != nil {
		t.Fatalf("the API document must be readable: %v", err)
	}
	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	// Parsing rather than grepping: a document that no longer parses is a
	// Swagger UI that shows nothing, and that is worth failing on by itself.
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("the API document is not valid YAML: %v", err)
	}

	undocumented := []string{}
	for path, methods := range registered {
		operations, ok := document.Paths[path]
		if !ok {
			for _, method := range methods {
				undocumented = append(undocumented, strings.ToUpper(method)+" "+path)
			}
			continue
		}
		for _, method := range methods {
			if _, ok := operations[method]; !ok {
				undocumented = append(undocumented, strings.ToUpper(method)+" "+path)
			}
		}
	}
	sort.Strings(undocumented)
	if len(undocumented) > 0 {
		shown := undocumented
		if len(shown) > 15 {
			shown = shown[:15]
		}
		t.Fatalf("%d route(s) are served and not described in static/openapi.yaml:\n  %s\n"+
			"run scripts/ops/generate-openapi.py to add them, then write a real summary for each",
			len(undocumented), strings.Join(shown, "\n  "))
	}
}

// The other direction: a document that describes an endpoint the platform does
// not serve sends a client to a 404 and makes them doubt the rest of it.
func TestTheAPIDocumentDescribesNothingThatIsNotServed(t *testing.T) {
	registration := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|DELETE|PATCH) ([^"]+)"`)
	sources, _ := filepath.Glob("*.go")
	served := map[string]struct{}{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		body, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range registration.FindAllStringSubmatch(string(body), -1) {
			served[strings.ReplaceAll(match[2], "...", "")] = struct{}{}
		}
	}

	raw, err := os.ReadFile(filepath.Join("static", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}

	// Community builds do not register the Enterprise routes, and the document
	// is shared: an Enterprise path missing here is expected, not a defect.
	phantom := []string{}
	for path := range document.Paths {
		if _, ok := served[path]; ok {
			continue
		}
		if strings.Contains(path, "/admin/backups") || strings.Contains(path, "/egress") ||
			strings.Contains(path, "/assistant") || strings.Contains(path, "/admin/audit") {
			continue
		}
		phantom = append(phantom, path)
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Errorf("the document describes %d path(s) nothing serves: %s", len(phantom), strings.Join(phantom, ", "))
	}
}
