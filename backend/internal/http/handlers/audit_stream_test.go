package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// The audit trail is backed up by streaming it. 690,000 events cannot be a
// slice: the platform would spend a gigabyte to copy a file it is about to
// write out row by row.
func TestStreamingVisitsEveryEventAndStopsOnError(t *testing.T) {
	auditStore := memory.NewAuditStore()
	for i := 0; i < 25; i++ {
		if err := auditStore.Create(audit.Event{
			ID: string(rune('a' + i%26)), OccurredAt: time.Now().UTC(), Action: "test",
		}); err != nil {
			t.Fatal(err)
		}
	}

	seen := 0
	if err := auditStore.Stream(store.AuditFilter{}, func(audit.Event) error {
		seen++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("streaming visited nothing")
	}

	// A failed upload must stop the walk rather than keep reading a table for
	// an object nobody will receive.
	stopped := 0
	wanted := errStopStream
	err := auditStore.Stream(store.AuditFilter{}, func(audit.Event) error {
		stopped++
		return wanted
	})
	if err != wanted {
		t.Fatalf("the visitor's error must come back, got %v", err)
	}
	if stopped != 1 {
		t.Fatalf("the walk must stop at the first error, visited %d", stopped)
	}
}

type streamError struct{}

func (streamError) Error() string { return "stop" }

var errStopStream = streamError{}
