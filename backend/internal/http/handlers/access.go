package handlers

import (
	"net/http"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

const userHeader = "X-Noryx-User"

func (h Handlers) requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := r.Header.Get(userHeader)
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Noryx-User header"})
		return "", false
	}
	return userID, true
}

func (h Handlers) requireProjectRole(
	w http.ResponseWriter,
	projectID string,
	userID string,
	check func(access.Role) bool,
	action string,
) bool {
	role, ok := h.accessStore.GetRole(projectID, userID)
	if !ok || !check(role) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role for " + action})
		return false
	}
	return true
}

func (h Handlers) projectExists(projectID string) (bool, error) {
	projects, err := h.projectStore.List()
	if err != nil {
		return false, err
	}
	for _, p := range projects {
		if p.ID == projectID {
			return true, nil
		}
	}
	return false, nil
}
