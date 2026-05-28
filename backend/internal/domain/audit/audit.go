package audit

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID           string         `json:"id"`
	OccurredAt   time.Time      `json:"occurredAt"`
	ActorUserID  string         `json:"actorUserId"`
	ActorIP      string         `json:"actorIp"`
	ActorAgent   string         `json:"actorUserAgent"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resourceType"`
	ResourceID   string         `json:"resourceId"`
	ProjectID    string         `json:"projectId,omitempty"`
	Outcome      string         `json:"outcome"`
	ErrorCode    string         `json:"errorCode,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

func New(actorUserID, actorIP, actorAgent, action, resourceType, resourceID, projectID, outcome, errorCode string, details map[string]any) Event {
	return Event{
		ID:           uuid.NewString(),
		OccurredAt:   time.Now().UTC(),
		ActorUserID:  actorUserID,
		ActorIP:      actorIP,
		ActorAgent:   actorAgent,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ProjectID:    projectID,
		Outcome:      outcome,
		ErrorCode:    errorCode,
		Details:      details,
	}
}
