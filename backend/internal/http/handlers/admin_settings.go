package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
)

// Platform settings API.
//
// Every value here used to require editing a manifest and rolling the
// deployment, which put configuration in three places that drift apart
// (ADR-034 follow-up). The registry in internal/settings declares what may be
// changed; anything undeclared is refused rather than stored, so the
// configuration surface stays auditable.

func (h Handlers) ListPlatformSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "settings"); !ok {
		return
	}
	if h.settings == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": h.settings.Effective()})
}

func (h Handlers) UpdatePlatformSetting(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "settings")
	if !ok {
		return
	}
	if h.settings == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "no settings store on this instance"})
		return
	}

	key := strings.TrimSpace(r.PathValue("key"))
	definition, found := settings.Lookup(key)
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown setting: " + key})
		return
	}

	var payload struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	if err := h.settings.Set(key, payload.Value, identity.UserID()); err != nil {
		// Validation messages are written for an administrator to act on, so
		// they are returned verbatim rather than flattened to "bad request".
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	recorded := payload.Value
	if definition.Secret {
		recorded = "********"
	}
	h.emitAudit(r, identity.UserID(), "settings.update", "platform_setting", key, "", "success", "", map[string]any{
		"value": recorded,
	})

	writeJSON(w, http.StatusOK, map[string]any{"items": h.settings.Effective()})
}
