package handlers

import (
	"net/http"
	"strings"
)

type mutationAuditWriter struct {
	http.ResponseWriter
	status int
}

func (w *mutationAuditWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *mutationAuditWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (w *mutationAuditWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// AuditMutations guarantees an audit trail for every API mutation without
// recording request payloads, which may contain secrets or file contents.
func (h Handlers) AuditMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuditedMutation(r) {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &mutationAuditWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		outcome := "success"
		errorCode := ""
		if recorder.status < 200 || recorder.status >= 400 {
			outcome = "failure"
			errorCode = http.StatusText(recorder.status)
		}
		resourceType, resourceID, projectID := mutationAuditResource(r)
		h.emitAudit(r, h.auditActorUserID(r), mutationAuditAction(r.Method), resourceType, resourceID, projectID, outcome, errorCode, map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
			"route":  r.Pattern,
			"status": recorder.status,
		})
	})
}

func isAuditedMutation(r *http.Request) bool {
	if r == nil || !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func mutationAuditAction(method string) string {
	switch method {
	case http.MethodPost:
		return "api.mutation.create"
	case http.MethodDelete:
		return "api.mutation.delete"
	default:
		return "api.mutation.update"
	}
}

func (h Handlers) auditActorUserID(r *http.Request) string {
	if userID, ok := h.userIDFromSessionOrBearerNoWrite(r); ok {
		return userID
	}
	return strings.TrimSpace(r.Header.Get(userHeader))
}

func mutationAuditResource(r *http.Request) (string, string, string) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	resourceType := "api"
	if len(parts) > 2 && parts[2] != "" {
		resourceType = strings.TrimSuffix(parts[2], "s")
	}
	resourceID := firstPathValue(r, "projectID", "datasetID", "repositoryID", "datasourceID", "workspaceID", "jobID", "cronJobID", "appID", "dashboardID", "buildID", "environmentID", "organizationID", "executionID", "name")
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	return resourceType, resourceID, projectID
}

func firstPathValue(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(r.PathValue(name)); value != "" {
			return value
		}
	}
	return ""
}
