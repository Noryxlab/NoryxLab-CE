package postgres

import (
	"regexp"
	"strings"
	"testing"
)

// A table has to exist before anything is added to it.
//
// This is the defect it catches, which shipped and was found by a first
// install: an `ALTER TABLE agent_runs ADD COLUMN question` sat above the
// `CREATE TABLE agent_runs` that makes it possible. Every installation that
// already had the table applied it without complaint, so two clusters and
// every developer machine were silent - and the platform refused to start on
// an empty database, which is the only case nobody tries twice.
//
// Needs no database on purpose: the failure is in the order of a list, and a
// test that needs Postgres to see it is a test that skips on the machine where
// the mistake is made.
func TestEveryAlteredTableIsCreatedFirst(t *testing.T) {
	created := map[string]int{}
	createRE := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
	alterRE := regexp.MustCompile(`(?is)ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
	indexRE := regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?[a-z_][a-z0-9_]*\s+ON\s+([a-z_][a-z0-9_]*)`)

	for position, statement := range migrationStatements() {
		if match := createRE.FindStringSubmatch(statement); match != nil {
			if _, seen := created[match[1]]; !seen {
				created[match[1]] = position
			}
			continue
		}
		for _, check := range []struct {
			what string
			re   *regexp.Regexp
		}{{"altered", alterRE}, {"indexed", indexRE}} {
			match := check.re.FindStringSubmatch(statement)
			if match == nil {
				continue
			}
			table := match[1]
			if _, ok := created[table]; !ok {
				t.Errorf("statement %d %s %q before it is created: %s",
					position, check.what, table, firstLineOf(statement))
			}
		}
	}
}

func firstLineOf(statement string) string {
	line := strings.TrimSpace(strings.SplitN(statement, "\n", 2)[0])
	if len(line) > 90 {
		return line[:90] + "..."
	}
	return line
}
