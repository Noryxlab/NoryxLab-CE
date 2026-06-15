package datasource

import (
	"time"

	"github.com/google/uuid"
)

type Datasource struct {
	ID                  string    `json:"id"`
	OwnerUserID         string    `json:"ownerUserId"`
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
		ID:             uuid.NewString(),
		OwnerUserID:    ownerUserID,
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
	return []ServiceDefinition{
		{
			ID: "postgresql-17", Name: "PostgreSQL 17", Type: "postgres",
			Image:      "harbor.lan/noryx-dataservices/postgresql@sha256:abe10a9e2631b6d60ba6fc4d5943ee39e7384da7b50ddc076fec5d066cc416ee",
			Dockerfile: "FROM postgres:17-alpine\n", System: true, DefaultPort: 5432,
			Description: "Base relationnelle PostgreSQL maintenue par la plateforme.",
		},
		{
			ID: "mysql-8", Name: "MySQL 8", Type: "mysql",
			Image:      "harbor.lan/noryx-dataservices/mysql@sha256:21635514702426e031ef50302c5dd738c2cbc990d02664de88a839b64daf63cc",
			Dockerfile: "FROM mysql:8.4\n", System: true, DefaultPort: 3306,
			Description: "Base relationnelle MySQL maintenue par la plateforme.",
		},
		{
			ID: "mongodb-8", Name: "MongoDB 8", Type: "mongodb",
			Image:      "harbor.lan/noryx-dataservices/mongodb@sha256:24b556c65745f3c67a6c5b89f65200df7f64ad718a09aed4c2d7359ecad83ad4",
			Dockerfile: "FROM mongo:8.0\n", System: true, DefaultPort: 27017,
			Description: "Base documentaire MongoDB maintenue par la plateforme.",
		},
	}
}
