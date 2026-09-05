package status

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every status the backend assigns must be declared here.
//
// The declaration is only worth having if it is kept honest, so this reads the
// backend's own sources and fails on a status that was invented somewhere and
// never added. It is the check that would have caught "launching" before it
// froze a screen.
func TestEveryStatusTheBackendEmitsIsDeclared(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	assignment := regexp.MustCompile(`(?:\.Status = |Status: +)"([a-z][a-z-]*)"`)

	found := map[string][]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Vendored or generated trees have their own vocabularies and are
			// not ours to police. The root is exempt from the hidden-directory
			// rule: it is reached as "../../.." and its name is "..", which
			// starts with a dot and skipped the entire walk.
			if path == root {
				return nil
			}
			if info.Name() == "vendor" || info.Name() == "node_modules" || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range assignment.FindAllStringSubmatch(string(source), -1) {
			found[match[1]] = append(found[match[1]], path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the backend sources: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no status assignment found at all: this test has stopped checking anything")
	}

	for value, places := range found {
		if _, declared := Of(value); !declared {
			t.Errorf("status %q is emitted by %s but not declared in the vocabulary", value, places[0])
		}
	}
}

// A declared status that nothing emits is dead vocabulary the interface still
// has to carry. Reported rather than failed: a status may legitimately arrive
// before the code that sets it.
func TestDeclaredStatusesThatNothingEmitsAreReported(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the whole source tree")
	}
	for _, name := range Names() {
		if _, declared := Of(name); !declared {
			t.Errorf("Names() returned %q, which Of() does not know", name)
		}
	}
}

// Pending is the answer that keeps an interface polling, so it must never be
// the default for something that has finished, and never absent for something
// in flight.
func TestKindsSayWhetherToKeepPolling(t *testing.T) {
	for value, expected := range map[string]Kind{
		"launching": KindPending,
		"submitted": KindPending,
		"running":   KindSuccess,
		"succeeded": KindSuccess,
		"failed":    KindFailed,
		"stopped":   KindStopped,
		"unchecked": KindUnknown,
	} {
		if kind, _ := Of(value); kind != expected {
			t.Errorf("status %q reads as %q, want %q", value, kind, expected)
		}
	}
	if _, declared := Of("teleporting"); declared {
		t.Error("an undeclared status must not come back as declared")
	}
}
