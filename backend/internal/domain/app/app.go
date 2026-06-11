package app

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type App struct {
	ID                   string     `json:"id"`
	ProjectID            string     `json:"projectId"`
	OwnerUserID          string     `json:"ownerUserId"`
	Kind                 string     `json:"kind"`
	Name                 string     `json:"name"`
	Slug                 string     `json:"slug"`
	Image                string     `json:"image"`
	Command              []string   `json:"command"`
	Args                 []string   `json:"args"`
	Port                 int        `json:"port"`
	PodName              string     `json:"podName"`
	ServiceName          string     `json:"serviceName"`
	Status               string     `json:"status"`
	AccessURL            string     `json:"accessUrl"`
	AccessMode           string     `json:"accessMode"`
	AllowedUsers         []string   `json:"allowedUsers,omitempty"`
	AllowedOrganizations []string   `json:"allowedOrganizations,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	HealthMessage        string     `json:"healthMessage,omitempty"`
	RestartCount         int        `json:"restartCount"`
	StartedAt            *time.Time `json:"startedAt,omitempty"`
	Published            bool       `json:"published"`
	ActiveRevision       int        `json:"activeRevision,omitempty"`
	PublishedAt          *time.Time `json:"publishedAt,omitempty"`
}

type Revision struct {
	ID              string          `json:"id"`
	AppID           string          `json:"appId"`
	Number          int             `json:"number"`
	Snapshot        App             `json:"snapshot"`
	RuntimeManifest json.RawMessage `json:"-"`
	PublishedBy     string          `json:"publishedBy"`
	PublishedAt     time.Time       `json:"publishedAt"`
	Active          bool            `json:"active"`
}

func NewRevision(item App, number int, runtimeManifest json.RawMessage, publishedBy string) Revision {
	return Revision{
		ID:              uuid.NewString(),
		AppID:           item.ID,
		Number:          number,
		Snapshot:        item,
		RuntimeManifest: runtimeManifest,
		PublishedBy:     publishedBy,
		PublishedAt:     time.Now().UTC(),
		Active:          true,
	}
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
