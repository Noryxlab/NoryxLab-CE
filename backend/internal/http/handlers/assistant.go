package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

type openAICompatibleChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
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

	var request openAICompatibleChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat completion request"})
		return
	}
	message := openAICompatibleMessagesToText(request.Messages)
	if message == "" || len(message) > 64000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid messages are required"})
		return
	}

	payload, _ := json.Marshal(assistantServiceRequest{
		UserID:      claims.UserID,
		ProjectID:   claims.ProjectID,
		WorkspaceID: claims.WorkspaceID,
		Surface:     "developer",
		Message:     message,
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
		h.emitAdvancedAudit(r, claims.UserID, "assistant.developer.chat", "assistant", claims.WorkspaceID, claims.ProjectID, "failure", "provider_unavailable", nil)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider is unavailable"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		h.emitAdvancedAudit(r, claims.UserID, "assistant.developer.chat", "assistant", claims.WorkspaceID, claims.ProjectID, "failure", "assistant_error", nil)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider is unavailable"})
		return
	}
	var result map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, response.Body, 1<<20)).Decode(&result); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "assistant provider returned an invalid response"})
		return
	}
	model, _ := result["model"].(string)
	if model == "" {
		model = strings.TrimSpace(request.Model)
	}
	answer, _ := result["message"].(string)
	h.emitAdvancedAudit(r, claims.UserID, "assistant.developer.chat", "assistant", claims.WorkspaceID, claims.ProjectID, "success", "", map[string]any{"surface": "developer", "model": model})
	if request.Stream {
		writeOpenAICompatibleStream(w, model, answer)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      fmt.Sprintf("chatcmpl-noryx-%d", time.Now().UTC().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().UTC().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]string{
				"role":    "assistant",
				"content": answer,
			},
			"finish_reason": "stop",
		}},
	})
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

func writeOpenAICompatibleStream(w http.ResponseWriter, model, answer string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	id := fmt.Sprintf("chatcmpl-noryx-%d", time.Now().UTC().UnixNano())
	writeOpenAICompatibleStreamChunk(w, id, model, map[string]string{"role": "assistant"}, nil)
	writeOpenAICompatibleStreamChunk(w, id, model, map[string]string{"content": answer}, nil)
	finishReason := "stop"
	writeOpenAICompatibleStreamChunk(w, id, model, map[string]string{}, &finishReason)
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeOpenAICompatibleStreamChunk(w http.ResponseWriter, id, model string, delta map[string]string, finishReason *string) {
	chunk := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().UTC().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         delta,
			"finish_reason": finishReason,
		}},
	}
	encoded, _ := json.Marshal(chunk)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
}

func openAICompatibleMessagesToText(messages []struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}) string {
	var b strings.Builder
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(openAICompatibleContentToText(message.Content))
		if content == "" {
			continue
		}
		if role == "" {
			role = "user"
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(role)
		b.WriteString(":\n")
		b.WriteString(content)
	}
	return strings.TrimSpace(b.String())
}

func openAICompatibleContentToText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := block["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
