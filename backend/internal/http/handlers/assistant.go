package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
)

type assistantChatRequest struct {
	ConversationID string `json:"conversationId"`
	ProjectID      string `json:"projectId"`
	WorkspaceID    string `json:"workspaceId"`
	Surface        string `json:"surface"`
	Message        string `json:"message"`
}

type assistantServiceRequest struct {
	ConversationID string `json:"conversationId"`
	UserID         string `json:"userId"`
	OrganizationID string `json:"organizationId"`
	ProjectID      string `json:"projectId"`
	WorkspaceID    string `json:"workspaceId"`
	Surface        string `json:"surface"`
	Message        string `json:"message"`
}

func (h Handlers) ChatWithAssistant(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.featureEnabled(edition.FeatureAssistant) || h.assistantURL == "" || h.assistantInternalToken == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "assistant is not enabled"})
		return
	}
	var request assistantChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid assistant request"})
		return
	}
	request.Message = strings.TrimSpace(request.Message)
	request.Surface = strings.TrimSpace(request.Surface)
	if request.Message == "" || len(request.Message) > 32000 || request.Surface != "platform" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "platform surface and a valid message are required"})
		return
	}
	// Project and workspace context will only be enabled once it is resolved and
	// authorized server-side. Never accept a browser-provided scope as trusted.
	if strings.TrimSpace(request.ProjectID) != "" || strings.TrimSpace(request.WorkspaceID) != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scoped assistant context is not available in this pilot"})
		return
	}
	organizationID := ""
	if h.keycloak != nil {
		if organizations, err := h.keycloak.ListUserOrganizations(identity.UserID()); err == nil && len(organizations) > 0 {
			organizationID = organizations[0].ID
		}
	}
	payload, _ := json.Marshal(assistantServiceRequest{
		ConversationID: request.ConversationID,
		UserID:         identity.UserID(),
		OrganizationID: organizationID,
		ProjectID:      "",
		WorkspaceID:    "",
		Surface:        request.Surface,
		Message:        request.Message,
	})
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()
	proxyRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, h.assistantURL+"/v1/chat", bytes.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant service is unavailable"})
		return
	}
	proxyRequest.Header.Set("Content-Type", "application/json")
	proxyRequest.Header.Set("X-Noryx-Assistant-Token", h.assistantInternalToken)
	response, err := (&http.Client{Timeout: 55 * time.Second}).Do(proxyRequest)
	if err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "assistant.chat", "assistant", "", request.ProjectID, "failure", "provider_unavailable", nil)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider is unavailable"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		h.emitAdvancedAudit(r, identity.UserID(), "assistant.chat", "assistant", "", request.ProjectID, "failure", "assistant_error", nil)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider is unavailable"})
		return
	}
	var result map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, response.Body, 1<<20)).Decode(&result); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider returned an invalid response"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "assistant.chat", "assistant", "", request.ProjectID, "success", "", map[string]any{"surface": request.Surface, "provider": result["provider"], "model": result["model"]})
	writeJSON(w, http.StatusOK, result)
}
