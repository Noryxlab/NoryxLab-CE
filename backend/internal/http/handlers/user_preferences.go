package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

const userPrefLanguageKey = "language"

type updateUserPreferencesRequest struct {
	Language string `json:"language"`
}

func normalizeLanguage(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	switch lang {
	case "fr", "fr-fr", "fr_ca", "fr-ca":
		return "fr"
	case "en", "en-us", "en_gb", "en-gb":
		return "en"
	default:
		return ""
	}
}

func (h Handlers) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	value, found, err := h.userPreferenceStore.Get(userID, userPrefLanguageKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read user preferences"})
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	lang := normalizeLanguage(value)
	if lang == "" {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"language": lang})
}

func (h Handlers) UpdateUserPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req updateUserPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	lang := normalizeLanguage(req.Language)
	if lang == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "language must be fr or en"})
		return
	}
	if err := h.userPreferenceStore.Set(userID, userPrefLanguageKey, lang); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user preferences"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"language": lang})
}

