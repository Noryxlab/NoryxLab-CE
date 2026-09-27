package app

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type App struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	// OwnerUserID is who created it, kept as it was.
	OwnerUserID string `json:"ownerUserId"`
	// OwnerType and OwnerID are who answers for it, which is not the same
	// question and until ADR-039 had no way of being asked: an app could only
	// belong to the person who launched it, so a production went unowned the
	// day they left. The pair matches what datasets and projects already
	// carry - "user", "organization", or a service account, which is a user
	// the directory marks as not being a person.
	OwnerType            string     `json:"ownerType,omitempty"`
	OwnerID              string     `json:"ownerId,omitempty"`
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
	// HardwareTier is what the app was launched with. The column has existed
	// since apps did and was never written or read, so a running application
	// held a pod that counted for nothing in its project's consumption or its
	// quota - an app was free, and only an app.
	HardwareTier string `json:"hardwareTier,omitempty"`
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
