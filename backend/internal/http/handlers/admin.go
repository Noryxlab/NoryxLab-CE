package handlers

import (
	"net/http"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

func (h Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}

	users, err := h.keycloak.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to fetch users from keycloak: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

func (h Handlers) GetModulesStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}

	inspector, ok := h.runtime.(noryxruntime.Inspector)
	if !ok || inspector == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "runtime inspector is not available"})
		return
	}

	deployments, err := inspector.ListDeployments()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list deployments: " + err.Error()})
		return
	}
	services, err := inspector.ListServices()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list services: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deployments": deployments,
		"services":    services,
	})
}
