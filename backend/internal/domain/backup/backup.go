package backup

import (
	"time"

	"github.com/google/uuid"
)

type Run struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	CreatedBy string     `json:"createdBy"`
	Bucket    string     `json:"bucket"`
	Prefix    string     `json:"prefix"`
	ObjectKey string     `json:"objectKey"`
	Report    string     `json:"report,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

func NewRun(createdBy, bucket, prefix string) Run {
	return Run{
		ID:        uuid.NewString(),
		Status:    "running",
		CreatedBy: createdBy,
		Bucket:    bucket,
		Prefix:    prefix,
		StartedAt: time.Now().UTC(),
	}
}
