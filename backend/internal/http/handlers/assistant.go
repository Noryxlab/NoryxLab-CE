package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func (h Handlers) ChatCompletionsWithDeveloperAssistant(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(edition.FeatureAssistant) || h.assistantURL == "" || h.assistantInternalToken == "" || h.assistantDeveloperSigningKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "assistant is not enabled"})
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	claims, err := verifyDeveloperAssistantToken(h.assistantDeveloperSigningKey, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid assistant token"})
		return
	}
	workspaceRecord, found, err := h.workspaceStore.GetByID(claims.WorkspaceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify workspace"})
		return
	}
	if !found || workspaceRecord.ProjectID != claims.ProjectID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "workspace is not authorized for this assistant token"})
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat completion request"})
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat completion request"})
		return
	}
	if !openAICompatiblePayloadHasMessages(payload) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid messages are required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 130*time.Second)
	defer cancel()
	proxyRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, h.assistantURL+"/v1/openai/chat/completions", bytes.NewReader(rawBody))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant service is unavailable"})
		return
	}
	proxyRequest.Header.Set("Content-Type", "application/json")
	proxyRequest.Header.Set("X-Noryx-Assistant-Token", h.assistantInternalToken)
	if accept := strings.TrimSpace(r.Header.Get("Accept")); accept != "" {
		proxyRequest.Header.Set("Accept", accept)
	}
	response, err := (&http.Client{Timeout: 135 * time.Second}).Do(proxyRequest)
	if err != nil {
		h.emitAdvancedAudit(r, claims.UserID, "assistant.developer.chat", "assistant", claims.WorkspaceID, claims.ProjectID, "failure", "provider_unavailable", nil)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider is unavailable"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		h.emitAdvancedAudit(r, claims.UserID, "assistant.developer.chat", "assistant", claims.WorkspaceID, claims.ProjectID, "failure", "assistant_error", nil)
		copyProxyResponse(w, response)
		return
	}
	h.emitAdvancedAudit(r, claims.UserID, "assistant.developer.chat", "assistant", claims.WorkspaceID, claims.ProjectID, "success", "", map[string]any{"surface": "developer"})
	copyProxyResponse(w, response)
}

func (h Handlers) ListDeveloperAssistantModels(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(edition.FeatureAssistant) || h.assistantDeveloperSigningKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "assistant is not enabled"})
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if _, err := verifyDeveloperAssistantToken(h.assistantDeveloperSigningKey, token); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid assistant token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{{
			"id":       "noryx-workspace",
			"object":   "model",
			"created":  0,
			"owned_by": "noryx",
		}},
	})
}

func openAICompatiblePayloadHasMessages(payload map[string]any) bool {
	messages, ok := payload["messages"].([]any)
	return ok && len(messages) > 0
}

func copyProxyResponse(w http.ResponseWriter, response *http.Response) {
	for _, header := range []string{"Content-Type", "Cache-Control"} {
		if value := strings.TrimSpace(response.Header.Get(header)); value != "" {
			w.Header().Set(header, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}
