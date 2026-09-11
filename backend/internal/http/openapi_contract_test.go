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
			strings.Contains(path, "/assistant") || strings.Contains(path, "/admin/audit") ||
			strings.Contains(path, "/agents") {
			continue
		}
		phantom = append(phantom, path)
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Errorf("the document describes %d path(s) nothing serves: %s", len(phantom), strings.Join(phantom, ", "))
	}
}

// Every reference resolves.
//
// A generated $ref pointing at a schema that was never inserted produces a
// document that parses cleanly and breaks Swagger UI the moment it loads -
// which is how the schema pass failed silently the first time it ran: it
// looked for `schemas:` at the wrong indentation, added nothing, and left a
// hundred references pointing at nothing.
func TestEveryReferenceInTheAPIDocumentResolves(t *testing.T) {
	// Both documents: the supported one is derived, and a derivation that
	// drops a schema somebody still points at breaks Swagger UI on load with
	// no error anywhere.
	for _, name := range []string{"openapi.yaml", "openapi.public.yaml"} {
		t.Run(name, func(t *testing.T) { assertReferencesResolve(t, name) })
	}
}

func assertReferencesResolve(t *testing.T, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("static", name))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Components struct {
			Schemas   map[string]any `yaml:"schemas"`
			Responses map[string]any `yaml:"responses"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}

	reference := regexp.MustCompile(`#/components/(schemas|responses)/(\w+)`)
	dangling := map[string]struct{}{}
	for _, match := range reference.FindAllStringSubmatch(string(raw), -1) {
		var known bool
		switch match[1] {
		case "schemas":
			_, known = document.Components.Schemas[match[2]]
		case "responses":
			_, known = document.Components.Responses[match[2]]
		}
		if !known {
			dangling[match[1]+"/"+match[2]] = struct{}{}
		}
	}
	if len(dangling) > 0 {
		names := make([]string, 0, len(dangling))
		for name := range dangling {
			names = append(names, name)
		}
		sort.Strings(names)
		t.Fatalf("%d reference(s) point at nothing: %s", len(names), strings.Join(names, ", "))
	}
}

// An operation that answers 200 and describes no body tells a client the call
// succeeds and nothing about what comes back. Proxies and file downloads are
// exempt: they return whatever the workspace or the object store returns.
func TestEverySupportedOperationDescribesWhatItReturns(t *testing.T) {
	// The supported document is the contract. An operation in it that answers
	// 200 and says nothing about the body is a route somebody can call and
	// nobody can generate a client for - which is how the API ended up with 42
	// of them while the document looked complete.
	raw, err := os.ReadFile(filepath.Join("static", "openapi.public.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]any `yaml:"content"`
			} `yaml:"responses"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}

	for path, operations := range document.Paths {
		for method, operation := range operations {
			response, ok := operation.Responses["200"]
			if !ok || len(response.Content) > 0 {
				continue
			}
			t.Errorf("%s %s answers 200 with no described body; declare it in RESPONSES "+
				"or OTHER_RESPONSES in scripts/ops/generate-openapi.py, or mark the "+
				"endpoint internal if only the interface calls it",
				strings.ToUpper(method), path)
		}
	}
}

func TestTheSupportedDocumentHoldsNothingInternal(t *testing.T) {
	// Two documents, one source: the supported one is the full one minus the
	// operations marked internal. If an internal operation appears here, the
	// interface's own endpoints have become somebody's integration.
	full := readAPIDocument(t, "openapi.yaml")
	public := readAPIDocument(t, "openapi.public.yaml")

	internal := 0
	for path, operations := range full {
		for method, operation := range operations {
			if !operation.Internal {
				if _, ok := public[path][method]; !ok {
					t.Errorf("%s %s is served and supported but missing from the supported document", strings.ToUpper(method), path)
				}
				continue
			}
			internal++
			if _, ok := public[path][method]; ok {
				t.Errorf("%s %s is marked internal and is in the supported document", strings.ToUpper(method), path)
			}
		}
	}
	for path, operations := range public {
		for method, operation := range operations {
			if operation.Internal {
				t.Errorf("%s %s carries x-noryx-internal in the supported document", strings.ToUpper(method), path)
			}
		}
	}
	if internal == 0 {
		t.Error("no operation is marked internal; the classification has been lost")
	}
}

type documentedOperation struct {
	Internal bool `yaml:"x-noryx-internal"`
}

func readAPIDocument(t *testing.T, name string) map[string]map[string]documentedOperation {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("static", name))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]documentedOperation `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return document.Paths
}
