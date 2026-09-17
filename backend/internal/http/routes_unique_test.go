package http

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Two handlers on the same route is a startup panic, not a compile error.
//
// Go's ServeMux refuses a duplicate pattern by panicking when it is
// registered, which happens as the process starts. So the build passes, the
// image is pushed, the rollout begins, and the platform crash-loops - the
// failure arrives at the worst possible moment and says nothing about which
// two routes collided.
//
// It happened on 2026-09-17: an activity report was registered on
// /api/v1/admin/usage, which had held the vCPU-hours report since June.
//
// Read from the source rather than by building a mux, because building one
// means constructing every dependency a handler needs, and a guard that is
// expensive to run is a guard that gets skipped.
func TestEveryRouteIsRegisteredOnce(t *testing.T) {
	file, err := os.Open("server.go")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	// mux.HandleFunc("GET /api/v1/...", h.Something)
	pattern := regexp.MustCompile(`mux\.Handle(?:Func)?\("([^"]+)"`)
	seen := map[string]int{}
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(text), "//") {
			continue
		}
		match := pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		route := match[1]
		if first, ok := seen[route]; ok {
			t.Errorf("%q is registered twice, at lines %d and %d: the platform would panic on startup", route, first, line)
			continue
		}
		seen[route] = line
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) == 0 {
		// The regexp stopped matching: the guard would pass forever without
		// checking anything, which is worse than not having it.
		t.Fatal("no routes found in server.go, the pattern no longer matches how routes are registered")
	}
	t.Logf("%d routes, all distinct", len(seen))
}
