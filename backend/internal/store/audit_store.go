package store

import (
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
)

type AuditFilter struct {
	Since       *time.Time
	Until       *time.Time
	Action      string
	ActorUserID string
	ResourceID  string
	ProjectID   string
	Limit       int
}

type AuditStore interface {
	Create(event audit.Event) error
	List(filter AuditFilter) ([]audit.Event, error)
	// Stream visits every matching event in order, without holding them all in
	// memory. A backup of 690,000 events cannot be a slice: the platform would
	// spend a gigabyte to copy a file it is about to write out row by row.
	//
	// visit returning an error stops the walk and returns it, so a failed
	// upload does not keep reading a table for nothing.
	Stream(filter AuditFilter, visit func(audit.Event) error) error
}
