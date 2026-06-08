package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

func (h Handlers) DeleteBuild(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	buildID := strings.TrimSpace(r.PathValue("buildID"))
	if buildID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "buildID is required"})
		return
	}

	record, found, err := h.buildStore.GetByID(buildID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read build"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "build not found"})
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanRunBuild, "build deletion") {
		return
	}

	if h.runtime != nil && strings.TrimSpace(record.JobName) != "" {
		if err := h.runtime.DeleteJob(record.JobName); err != nil && !isNotFoundError(err) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes build delete failed: " + err.Error()})
			return
		}
	}
	if err := h.buildStore.Delete(record.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete build"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.emitAudit(r, userID, "environment.build.delete", "build", record.ID, record.ProjectID, "success", "", map[string]any{
		"destinationImage": record.DestinationImage,
		"jobName":          record.JobName,
	})
}
