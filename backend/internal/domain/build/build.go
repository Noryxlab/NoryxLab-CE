package build

import (
	"time"

	"github.com/google/uuid"
)

type Build struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"projectId"`
	GitRepository    string    `json:"gitRepository"`
	GitRef           string    `json:"gitRef"`
	DockerfilePath   string    `json:"dockerfilePath"`
	ContextPath      string    `json:"contextPath"`
	DestinationImage string    `json:"destinationImage"`
	JobName          string    `json:"jobName"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
}

func New(projectID, gitRepo, gitRef, dockerfilePath, contextPath, destinationImage, jobName string) Build {
	return Build{
		ID:               uuid.NewString(),
		ProjectID:        projectID,
		GitRepository:    gitRepo,
		GitRef:           gitRef,
		DockerfilePath:   dockerfilePath,
		ContextPath:      contextPath,
		DestinationImage: destinationImage,
		JobName:          jobName,
		Status:           "submitted",
		CreatedAt:        time.Now().UTC(),
	}
}
