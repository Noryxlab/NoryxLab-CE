package workspace

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"projectId"`
	Kind         string    `json:"kind"`
	Name         string    `json:"name"`
	Image        string    `json:"image"`
	PodName      string    `json:"podName"`
	ServiceName  string    `json:"serviceName"`
	PVCName      string    `json:"pvcName"`
	PVCClass     string    `json:"pvcClass"`
	PVCSize      string    `json:"pvcSize"`
	PVCMountPath string    `json:"pvcMountPath"`
	CPU          string    `json:"cpu"`
	Memory       string    `json:"memory"`
	Status       string    `json:"status"`
	AccessURL    string    `json:"accessUrl"`
	AccessToken  string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
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

func New(kind, projectID, name, image, podName, serviceName, cpu, memory, accessURL, accessToken string) Workspace {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "jupyter"
	}
	return Workspace{
		ID:          uuid.NewString(),
		ProjectID:   projectID,
		Kind:        kind,
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
