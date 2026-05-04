package datasource

import (
	"time"

	"github.com/google/uuid"
)

type Datasource struct {
	ID             string    `json:"id"`
	OwnerUserID    string    `json:"ownerUserId"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	Database       string    `json:"database"`
	Username       string    `json:"username"`
	PasswordSecret string    `json:"passwordSecret"`
	SSLMode        string    `json:"sslMode"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func New(ownerUserID, name, kind, host, database, username, passwordSecret, sslMode string, port int) Datasource {
	now := time.Now().UTC()
	return Datasource{
		ID:             uuid.NewString(),
		OwnerUserID:    ownerUserID,
		Name:           name,
		Type:           kind,
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
