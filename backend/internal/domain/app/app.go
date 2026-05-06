package app

import (
	"time"

	"github.com/google/uuid"
)

type App struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Image       string    `json:"image"`
	Command     []string  `json:"command"`
	Args        []string  `json:"args"`
	Port        int       `json:"port"`
	PodName     string    `json:"podName"`
	ServiceName string    `json:"serviceName"`
	Status      string    `json:"status"`
	AccessURL   string    `json:"accessUrl"`
	CreatedAt   time.Time `json:"createdAt"`
}

func New(projectID, name, slug, image string, command, args []string, port int, podName, serviceName, accessURL string) App {
	return App{
		ID:          uuid.NewString(),
		ProjectID:   projectID,
		Kind:        "app",
		Name:        name,
		Slug:        slug,
		Image:       image,
		Command:     command,
		Args:        args,
		Port:        port,
		PodName:     podName,
		ServiceName: serviceName,
		Status:      "submitted",
		AccessURL:   accessURL,
		CreatedAt:   time.Now().UTC(),
	}
}

func NewWithKind(kind, projectID, name, slug, image string, command, args []string, port int, podName, serviceName, accessURL string) App {
	item := New(projectID, name, slug, image, command, args, port, podName, serviceName, accessURL)
	if kind != "" {
		item.Kind = kind
	}
	return item
}
