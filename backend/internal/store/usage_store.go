package store

import (
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
)

// UsageStore keeps what each project held, sampled over time.
type UsageStore interface {
	// Record stores one sweep: every project measured at the same instant.
	Record(samples []usage.Sample) error
	// ListByProject returns the samples for one project in a window, oldest
	// first, so they can be integrated in order.
	ListByProject(projectID string, from, to time.Time) ([]usage.Sample, error)
	// ListProjects returns the project ids that have any sample in a window,
	// which is what an administrator's "who consumed what" starts from.
	ListProjects(from, to time.Time) ([]string, error)
	// DeleteBefore drops old samples. Consumption history is not an audit
	// trail: it is kept to answer a question about a month, not forever.
	DeleteBefore(cutoff time.Time) (int64, error)
}
