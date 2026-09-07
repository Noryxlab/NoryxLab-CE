package build

import (
	"time"

	"github.com/google/uuid"
)

type Build struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"projectId"`
	GitRepository     string    `json:"gitRepository"`
	GitRef            string    `json:"gitRef"`
	DockerfilePath    string    `json:"dockerfilePath"`
	DockerfileContent string    `json:"dockerfileContent,omitempty"`
	ContextPath       string    `json:"contextPath"`
	DestinationImage  string    `json:"destinationImage"`
	JobName           string    `json:"jobName"`
	// Name is what a person called this environment. Without it the only
	// identity left is the image reference, and a screen has no choice but to
	// show "1cf6b279-114-test-stef:1788808618" to somebody who typed
	// "test-stef".
	Name string `json:"name,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
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
