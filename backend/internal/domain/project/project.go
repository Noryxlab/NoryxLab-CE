package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func New(name string) Project {
	return Project{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now().UTC(),
	}
}
