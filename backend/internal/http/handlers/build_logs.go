package handlers

import (
	"net/http"
	"strconv"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// What a build actually did.
//
// A build was a status and nothing else: an environment that failed to build
// said "failed", and the reason - a package that does not exist, a base image
// the cluster cannot pull, a typo on line three - stayed in a pod that the job
// then deleted. The person who wrote the Dockerfile had no way to find out
// what was wrong with it, which makes the feature unusable rather than
// imperfect.
func (h Handlers) GetBuildLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	buildID := strings.TrimSpace(r.PathValue("buildID"))
	record, found, err := h.buildStore.GetByID(buildID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read build"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "build not found"})
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, actionRunBuild, "build logs access") {
		return
	}

	logReader, ok := h.runtime.(noryxruntime.JobLogReader)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "build logs are not supported by this runtime"})
		return
	}
	tailLines := 400
	if raw := strings.TrimSpace(r.URL.Query().Get("tailLines")); raw != "" {
		if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 {
			tailLines = n
		}
	}

	logs, err := logReader.GetJobLogs(record.JobName, tailLines)
	if err != nil {
		// A build whose pod is gone still has something to say: the status it
		// ended on. Answering 200 with that beats a 502 that reads like the
		// platform is broken when the build simply finished a week ago.
		if record.Status == "running" || record.Status == "submitted" {
			writeJSON(w, http.StatusOK, map[string]any{
				"buildId": record.ID, "jobName": record.JobName, "projectId": record.ProjectID,
				"status": record.Status, "logs": "", "pending": true,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"buildId": record.ID, "jobName": record.JobName, "projectId": record.ProjectID,
			"status": record.Status, "logs": "",
			"unavailable": "the build's pod is gone, so its output is no longer available",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"buildId":   record.ID,
		"jobName":   record.JobName,
		"projectId": record.ProjectID,
		"status":    record.Status,
		"podName":   logs.PodName,
		"logs":      logs.Logs,
	})
}
