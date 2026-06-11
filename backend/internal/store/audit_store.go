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
}
