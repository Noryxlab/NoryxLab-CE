package handlers

import (
	"log"
	"net/http"
	"strings"
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
	if !h.requireProjectRole(w, projectID, userID, actionRunBuild, "environment deletion") {
		return
	}
	if destinationImage == strings.TrimSpace(h.workspaceJupyterImage) || destinationImage == strings.TrimSpace(h.workspaceVSCodeImage) || destinationImage == strings.TrimSpace(h.workspaceRStudioImage) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "system workspace environments cannot be deleted"})
		return
	}

	builds, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}

	// Compared by repository, not by full reference: an environment is one
	// repository and every rebuild pushed a new tag, so matching the reference
	// deleted the row and left every other revision behind.
	target := imageRepository(destinationImage)
	deleted := 0
	for _, b := range builds {
		if b.ProjectID != projectID || imageRepository(strings.TrimSpace(b.DestinationImage)) != target {
			continue
		}
		deleted++
		if h.runtime != nil && strings.TrimSpace(b.JobName) != "" {
			_ = h.runtime.DeleteJob(b.JobName)
		}
		_ = h.buildStore.Delete(b.ID)
	}

	if deleted == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no environment named " + destinationImage + " in this project"})
		return
	}

	// The registry copy is best effort: an environment removed from the
	// platform and left in Harbor is untidy, and a registry that refuses the
	// delete - no credentials, retention policy, another project sharing the
	// repository - must not leave the environment half-removed on a screen
	// that already said it was gone.
	if err := h.deleteImageFromHarbor(destinationImage); err != nil {
		log.Printf("environment %s removed; its registry image was kept: %v", destinationImage, err)
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
