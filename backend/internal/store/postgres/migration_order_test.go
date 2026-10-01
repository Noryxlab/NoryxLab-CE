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
			if _, ok := created[table]; ok {
				continue
			}
			// A rename is the one legitimate way to touch a table this
			// schema never creates: the old name exists only on
			// installations that predate the rename, and the statement
			// asks whether it is there before touching it.
			//
			// Recognised by that guard and nothing else. An unguarded
			// ALTER on a table nobody creates is still the defect this
			// test exists for, and the guard is exactly what makes the
			// difference between "does nothing on a fresh database" and
			// "refuses to start on one".
			if estUnRenommageGarde(statement, table) {
				continue
			}
			t.Errorf("statement %d %s %q before it is created: %s",
				position, check.what, table, firstLineOf(statement))
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

// estUnRenommageGarde reporte un ALTER protege par un test d existence sur la
// table qu il touche.
//
// to_regclass rend NULL pour une table absente, ce qui fait du bloc entier une
// instruction sans effet sur une base vierge - la seule situation que le test
// ci-dessus protege.
func estUnRenommageGarde(statement, table string) bool {
	if !strings.Contains(strings.ToUpper(statement), "RENAME TO") {
		return false
	}
	garde := regexp.MustCompile(`(?is)to_regclass\s*\(\s*'(?:public\.)?` +
		regexp.QuoteMeta(table) + `'\s*\)\s+IS\s+NOT\s+NULL`)
	return garde.MatchString(statement)
}

// Et un ALTER sans garde sur une table que ce schema ne cree pas reste le
// defaut que ce fichier existe pour attraper. Sans ce test, assouplir la regle
// pour le renommage l aurait assouplie pour tout le monde.
func TestUnAlterSansGardeResteRefuse(t *testing.T) {
	nu := `ALTER TABLE cohorts RENAME TO extracts`
	if estUnRenommageGarde(nu, "cohorts") {
		t.Error("un renommage sans test d existence a ete accepte")
	}
	garde := `DO $$ BEGIN
		IF to_regclass('public.cohorts') IS NOT NULL THEN
			ALTER TABLE cohorts RENAME TO extracts;
		END IF;
	END $$;`
	if !estUnRenommageGarde(garde, "cohorts") {
		t.Error("un renommage garde a ete refuse")
	}
	// La garde doit porter sur la bonne table : celle d a cote ne protege rien.
	ailleurs := `DO $$ BEGIN
		IF to_regclass('public.autre_table') IS NOT NULL THEN
			ALTER TABLE cohorts RENAME TO extracts;
		END IF;
	END $$;`
	if estUnRenommageGarde(ailleurs, "cohorts") {
		t.Error("une garde portant sur une autre table a ete acceptee")
	}
}
