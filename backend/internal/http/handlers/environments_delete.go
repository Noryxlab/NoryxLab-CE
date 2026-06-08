package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

func (h Handlers) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	envID := strings.TrimSpace(r.PathValue("environmentID"))
	projectID, destinationImage, ok := splitEnvironmentKey(envID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environmentID"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanRunBuild, "environment deletion") {
		return
	}

	builds, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}

	for _, b := range builds {
		if b.ProjectID != projectID || strings.TrimSpace(b.DestinationImage) != destinationImage {
			continue
		}
		if h.runtime != nil && strings.TrimSpace(b.JobName) != "" {
			_ = h.runtime.DeleteJob(b.JobName)
		}
		_ = h.buildStore.Delete(b.ID)
	}

	if err := h.deleteImageFromHarbor(destinationImage); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "registry image delete failed: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.emitAudit(r, userID, "environment.image.delete", "environment", envID, projectID, "success", "", map[string]any{
		"destinationImage": destinationImage,
	})
}
func splitEnvironmentKey(key string) (projectID, destinationImage string, ok bool) {
	raw := strings.TrimSpace(key)
	if raw == "" {
		return "", "", false
	}
	sep := strings.Index(raw, "|")
	if sep <= 0 || sep >= len(raw)-1 {
		return "", "", false
	}
	projectID = strings.TrimSpace(raw[:sep])
	destinationImage = strings.TrimSpace(raw[sep+1:])
	if projectID == "" || destinationImage == "" {
		return "", "", false
	}
	return projectID, destinationImage, true
}
