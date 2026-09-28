package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A setting has to reach the code that reads it.
//
// This is the defect it catches, found by launching rather than by reading:
// `NORYX_COHORT_CACHE_SIZE` was read by the configuration, carried on the
// configuration struct, accepted by the handlers' options and never assigned
// between the two - so an installation asking for an 8 GiB cache got the 50
// GiB default, and the volume would not schedule. Nothing failed: the value
// was simply the fallback, which is the failure that takes longest to see.
//
// The check is textual on purpose, and conservative: a name present on both
// structs must appear on the left of an assignment in this file. It says
// nothing about names that exist on only one of them.
func TestEveryConfigFieldWithAnOptionIsWired(t *testing.T) {
	config := fieldsOf(t, "../../internal/config/config.go", "Config")
	options := fieldsOf(t, "../../internal/http/handlers/handlers.go", "Options")
	wiring, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	text := string(wiring)

	shared, missing := 0, []string{}
	for name := range config {
		if !options[name] {
			continue
		}
		shared++
		if !strings.Contains(text, name+":") {
			missing = append(missing, name)
		}
	}
	if shared < 10 {
		t.Fatalf("only %d shared field(s) found; the struct patterns no longer match", shared)
	}
	if len(missing) > 0 {
		t.Fatalf("carried by the configuration and accepted by the handlers, assigned nowhere: %s",
			strings.Join(missing, ", "))
	}
}

func fieldsOf(t *testing.T, path, structName string) map[string]bool {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := string(source)
	start := strings.Index(text, "type "+structName+" struct {")
	if start < 0 {
		t.Fatalf("%s not found in %s", structName, path)
	}
	end := strings.Index(text[start:], "\n}")
	if end < 0 {
		t.Fatalf("%s is not closed in %s", structName, path)
	}
	names := map[string]bool{}
	fieldRE := regexp.MustCompile(`(?m)^\t([A-Z][A-Za-z0-9_]*)\s+[\w\[\]\*\.]+`)
	for _, match := range fieldRE.FindAllStringSubmatch(text[start:start+end], -1) {
		names[match[1]] = true
	}
	return names
}
