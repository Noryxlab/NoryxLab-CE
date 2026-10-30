package datasource

import (
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Access is a grant on one connector.
//
// The last catalogue object with no way to share: a connector was reachable
// by the person who declared it and by nobody else, so a team working on the
// same database each declared their own - the same host, the same credentials,
// copied by hand, and no way to answer who could reach what.
//
// Same shape as a dataset's, and the same two roles: a reader may use the
// connector in a workspace and see who else holds it, a writer may also
// change what it points at. Deciding who gets it stays with the owner.
//
// The second role was a promise with nothing behind it until 2026-10-30:
// there was no route that edited a connector, so Write and Read were the same
// right under two names, and a sweep that measured both found it. The route
// exists now, and the consequence it carries is why its audit entry names the
// fields that moved: a host changed here redirects every workspace that
// mounts this connector.
type Access struct {
	DatasourceID string    `json:"datasourceId"`
	SubjectType  string    `json:"subjectType"`
	SubjectID    string    `json:"subjectId"`
	SubjectName  string    `json:"subjectName,omitempty"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Datasource struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	// OwnerType and OwnerID carry the same meaning as on every other catalogue
	// object: who the connector belongs to, and therefore who answers for it.
	//
	// A database connector outlives the person who declared it more often than
	// anything else here - it points at a system the organization runs - and it
	// was the last object that could not be handed on.
	OwnerType string `json:"ownerType"`
	OwnerID   string `json:"ownerId"`
	// OwnerName is what a screen shows, resolved for a team or an organization
	// and equal to the username for a person.
	OwnerName           string    `json:"ownerName,omitempty"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	Source              string    `json:"source"`
	Host                string    `json:"host"`
	Port                int       `json:"port"`
	Database            string    `json:"database"`
	Username            string    `json:"username"`
	PasswordSecret      string    `json:"passwordSecret"`
	SSLMode             string    `json:"sslMode"`
	ServiceDefinitionID string    `json:"serviceDefinitionId,omitempty"`
	Image               string    `json:"image,omitempty"`
	Dockerfile          string    `json:"dockerfile,omitempty"`
	System              bool      `json:"system"`
	Status              string    `json:"status,omitempty"`
	PodName             string    `json:"podName,omitempty"`
	ServiceName         string    `json:"serviceName,omitempty"`
	PVCName             string    `json:"pvcName,omitempty"`
	StorageSize         string    `json:"storageSize,omitempty"`
	HardwareTier        string    `json:"hardwareTier,omitempty"`
	StatusReason        string    `json:"statusReason,omitempty"`
	StatusMessage       string    `json:"statusMessage,omitempty"`
	RestartCount        int       `json:"restartCount,omitempty"`
	StartedAt           time.Time `json:"startedAt,omitempty"`
	AttachedProjectIDs  []string  `json:"attachedProjectIds,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func Internal(ownerUserID, name, database, username, passwordSecret, storageSize, hardwareTier string, definition ServiceDefinition, podName, serviceName, pvcName string) Datasource {
	item := New(ownerUserID, name, definition.Type, serviceName+".noryx-loads.svc.cluster.local", database, username, passwordSecret, "disable", definition.DefaultPort)
	item.Source = "internal"
	item.ServiceDefinitionID = definition.ID
	item.Image = definition.Image
	item.Dockerfile = definition.Dockerfile
	item.System = definition.System
	item.Status = "launching"
	item.PodName = podName
	item.ServiceName = serviceName
	item.PVCName = pvcName
	item.StorageSize = storageSize
	item.HardwareTier = hardwareTier
	return item
}

func New(ownerUserID, name, kind, host, database, username, passwordSecret, sslMode string, port int) Datasource {
	now := time.Now().UTC()
	return Datasource{
		ID:          uuid.NewString(),
		OwnerUserID: ownerUserID,
		// Le meme defaut que partout ailleurs dans le catalogue : son auteur,
		// jusqu a ce que quelqu un le transfere.
		OwnerType:      "user",
		OwnerID:        ownerUserID,
		Name:           name,
		Type:           kind,
		Source:         "external",
		Host:           host,
		Port:           port,
		Database:       database,
		Username:       username,
		PasswordSecret: passwordSecret,
		SSLMode:        sslMode,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

type ServiceDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Image       string `json:"image"`
	Dockerfile  string `json:"dockerfile"`
	System      bool   `json:"system"`
	Description string `json:"description"`
	DefaultPort int    `json:"defaultPort"`
}

func SystemServiceDefinitions() []ServiceDefinition {
	registry := strings.TrimRight(strings.TrimSpace(os.Getenv("NORYX_DATASERVICE_REGISTRY")), "/")
	if registry == "" {
		registry = "harbor.example.local/noryx-dataservices"
	}
	return []ServiceDefinition{
		{
			ID: "postgresql-17", Name: "PostgreSQL 17", Type: "postgres",
			Image:      registry + "/postgresql:17",
			Dockerfile: "FROM postgres:17-alpine\n", System: true, DefaultPort: 5432,
			Description: "Base relationnelle PostgreSQL maintenue par la plateforme.",
		},
		{
			ID: "mysql-8", Name: "MySQL 8", Type: "mysql",
			Image:      registry + "/mysql:8.4",
			Dockerfile: "FROM mysql:8.4\n", System: true, DefaultPort: 3306,
			Description: "Base relationnelle MySQL maintenue par la plateforme.",
		},
		{
			ID: "mongodb-8", Name: "MongoDB 8", Type: "mongodb",
			Image:      registry + "/mongodb:8.0",
			Dockerfile: "FROM mongo:8.0\n", System: true, DefaultPort: 27017,
			Description: "Base documentaire MongoDB maintenue par la plateforme.",
		},
	}
}
