package postgres

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every column selected has somewhere to land.
//
// This is the defect it catches, which shipped to two installations on
// 2026-09-27: `owner_type` and `owner_id` were added to the apps listing's
// SELECT and to its two sibling queries, and the listing's Scan was left with
// its original twenty-two targets. Postgres was happy, the compiler was happy,
// and the application screen answered 500 with "failed to list apps" - the
// error `database/sql` raises at runtime and nowhere else.
//
// Needs no database on purpose, like the migration order test beside it: the
// mistake is a mismatch between two lists in one file, and a check that needs
// Postgres to see it is a check that skips on the machine where the mistake is
// made.
func TestEverySelectedColumnHasAScanTarget(t *testing.T) {
	source, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	text := string(source)

	// Only the queries this can read confidently: a plain column list, and a
	// Scan close enough after it to belong to it. A query this skips is not
	// checked, which is why the failure message names what it compared.
	selectRE := regexp.MustCompile("SELECT ((?:[a-z_][a-z0-9_]*)(?:,\\s*[a-z_][a-z0-9_]*)+) FROM ([a-z_]+)")
	argumentRE := regexp.MustCompile(`&[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+)?`)

	checked := 0
	for _, match := range selectRE.FindAllStringSubmatchIndex(text, -1) {
		columns := strings.Split(text[match[2]:match[3]], ",")
		table := text[match[4]:match[5]]
		if len(columns) < 4 {
			continue
		}
		// The window stops at whatever comes first after the query: another
		// SELECT, or the next function. A store that hands its rows to a
		// shared scanner - several do - has no Scan of its own in reach, and
		// this skips it rather than comparing against somebody else's.
		window := text[match[1]:min(match[1]+3000, len(text))]
		for _, boundary := range []string{"SELECT ", "\nfunc "} {
			if at := strings.Index(window, boundary); at > 0 {
				window = window[:at]
			}
		}
		scanAt := strings.Index(window, ".Scan(")
		if scanAt < 0 {
			continue
		}
		end := strings.Index(window[scanAt:], "); err")
		if end < 0 {
			continue
		}
		targets := argumentRE.FindAllString(window[scanAt:scanAt+end], -1)
		if len(targets) == 0 {
			continue
		}
		checked++
		if len(targets) != len(columns) {
			t.Errorf("%s: %d column(s) selected, %d scan target(s) - %s",
				table, len(columns), len(targets), strings.TrimSpace(strings.Join(columns, ",")))
		}
	}
	if checked < 5 {
		t.Fatalf("only %d query/scan pair(s) could be compared; the patterns above no longer match the file", checked)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
