package job

import (
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Command   []string  `json:"command"`
	Args      []string  `json:"args"`
	JobName   string    `json:"jobName"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func New(projectID, name, image, jobName string, command, args []string) Job {
	id := uuid.NewString()
	return Job{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Image:     image,
		Command:   command,
		Args:      args,
		JobName:   jobName,
		Status:    "submitted",
		CreatedAt: time.Now().UTC(),
	}
}
