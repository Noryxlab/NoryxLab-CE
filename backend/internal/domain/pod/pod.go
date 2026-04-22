package pod

import (
	"time"

	"github.com/google/uuid"
)

type Launch struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"projectId"`
	PodName   string            `json:"podName"`
	Image     string            `json:"image"`
	Command   []string          `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
}

func New(projectID, podName, image string, command, args []string, env map[string]string) Launch {
	return Launch{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		PodName:   podName,
		Image:     image,
		Command:   command,
		Args:      args,
		Env:       env,
		Status:    "submitted",
		CreatedAt: time.Now().UTC(),
	}
}
