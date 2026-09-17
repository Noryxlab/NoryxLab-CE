package memory

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
)

func at(day int, actor, action string) audit.Event {
	return audit.Event{
		ActorUserID: actor,
		Action:      action,
		Outcome:     "success",
		OccurredAt:  time.Date(2026, 9, day, 10, 0, 0, 0, time.UTC),
	}
}

// The decision the whole report rests on.
//
// A bulk import writes one audit row per object: one installation holds 690,000
// events, of which 685,000 are a single ontology import made by one person in
// one afternoon. Counted as events, that afternoon buries a week of real work
// and the chart describes the import rather than the platform.
func TestABulkImportDoesNotDecideTheShapeOfTheWeek(t *testing.T) {
	store := NewAuditStore()
	// Monday: one person, one enormous import.
	for i := 0; i < 5000; i++ {
		_ = store.Create(at(14, "importer", "api.mutation.create"))
	}
	// Tuesday: three people working normally.
	for _, who := range []string{"malinetc", "cridelic", "hamlaous"} {
		_ = store.Create(at(15, who, "job.launch"))
	}

	report, err := store.Usage(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Daily) != 2 {
		t.Fatalf("expected two days, got %d", len(report.Daily))
	}
	monday, tuesday := report.Daily[0], report.Daily[1]
	if monday.People != 1 {
		t.Errorf("the import day counted %d people, want 1", monday.People)
	}
	if tuesday.People <= monday.People {
		t.Errorf("a day with three people (%d) should outrank a day with one (%d); the series is counting events, not people",
			tuesday.People, monday.People)
	}
	// And the import is still visible for what it is, rather than filtered away.
	if report.Actions[0].Action != "api.mutation.create" || report.Actions[0].Count != 5000 {
		t.Errorf("the import should still be reported in full, got %+v", report.Actions[0])
	}
}

func TestCoverageIsReportedSeparatelyFromTheWindow(t *testing.T) {
	// A report over ninety days on records that begin twelve days ago is not a
	// quiet quarter. The screen can only say which it is if the store tells it
	// what it actually holds.
	store := NewAuditStore()
	_ = store.Create(at(15, "stef", "auth.login"))

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	report, _ := store.Usage(since, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if !report.Since.Equal(since) {
		t.Errorf("window start %s, want %s", report.Since, since)
	}
	if report.CoversSince.Equal(since) {
		t.Error("coverage was reported as the window rather than as what is held")
	}
	if report.CoversSince.Day() != 15 {
		t.Errorf("coverage starts %s, want the first event", report.CoversSince)
	}
}

func TestEventsOutsideTheWindowAreExcludedButStillCounted(t *testing.T) {
	store := NewAuditStore()
	_ = store.Create(at(1, "old", "job.launch"))
	_ = store.Create(at(20, "recent", "job.launch"))

	report, _ := store.Usage(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if report.TotalEvents != 1 {
		t.Errorf("counted %d events in the window, want 1", report.TotalEvents)
	}
	// The one outside still moves the coverage, which is how the screen knows
	// records exist further back than the period being shown.
	if report.CoversSince.Day() != 1 {
		t.Errorf("coverage ignored the older event: %s", report.CoversSince)
	}
}

func TestPeopleAreRankedByWhatTheyDid(t *testing.T) {
	store := NewAuditStore()
	for i := 0; i < 3; i++ {
		_ = store.Create(at(15, "busy", "job.launch"))
	}
	_ = store.Create(at(15, "quiet", "job.launch"))

	report, _ := store.Usage(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if len(report.People) != 2 || report.People[0].Actor != "busy" {
		t.Errorf("people not ranked by activity: %+v", report.People)
	}
}
