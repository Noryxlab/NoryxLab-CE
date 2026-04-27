package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
)

type upsertSecretRequest struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type secretView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (h Handlers) ListSecrets(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.secretStore.ListByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list secrets"})
		return
	}
	out := make([]secretView, 0, len(items))
	for _, item := range items {
		out = append(out, secretView{
			ID:        item.ID,
			Name:      item.Name,
			Type:      item.Type,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h Handlers) UpsertSecret(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req upsertSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)
	req.Value = strings.TrimSpace(req.Value)
	if req.Name == "" || req.Value == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and value are required"})
		return
	}
	if strings.TrimSpace(h.secretsMasterKey) == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "secrets encryption key is not configured"})
		return
	}
	encrypted, err := security.EncryptString(h.secretsMasterKey, req.Value)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt secret"})
		return
	}

	existing, found, err := h.secretStore.GetByName(userID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read secret"})
		return
	}
	if found {
		existing.ValueEncrypted = encrypted
		existing.UpdatedAt = time.Now().UTC()
		if req.Type != "" {
			existing.Type = req.Type
		}
		if err := h.secretStore.Upsert(existing); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update secret"})
			return
		}
		writeJSON(w, http.StatusOK, secretView{ID: existing.ID, Name: existing.Name, Type: existing.Type, CreatedAt: existing.CreatedAt, UpdatedAt: existing.UpdatedAt})
		return
	}
	item := secret.New(userID, req.Name, req.Type, encrypted)
	if err := h.secretStore.Upsert(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create secret"})
		return
	}
	writeJSON(w, http.StatusCreated, secretView{ID: item.ID, Name: item.Name, Type: item.Type, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
}

func (h Handlers) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "secret name is required"})
		return
	}
	if err := h.secretStore.Delete(userID, name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete secret"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
