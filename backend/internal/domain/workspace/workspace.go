package workspace

import (
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	PodName     string    `json:"podName"`
	ServiceName string    `json:"serviceName"`
	CPU         string    `json:"cpu"`
	Memory      string    `json:"memory"`
	Status      string    `json:"status"`
	AccessURL   string    `json:"accessUrl"`
	AccessToken string    `json:"accessToken"`
	CreatedAt   time.Time `json:"createdAt"`
}

func NewJupyter(projectID, name, image, podName, serviceName, cpu, memory, accessURL, accessToken string) Workspace {
	return Workspace{
		ID:          uuid.NewString(),
		ProjectID:   projectID,
		Kind:        "jupyter",
		Name:        name,
		Image:       image,
		PodName:     podName,
		ServiceName: serviceName,
		CPU:         cpu,
		Memory:      memory,
		Status:      "submitted",
		AccessURL:   accessURL,
		AccessToken: accessToken,
		CreatedAt:   time.Now().UTC(),
	}
}
