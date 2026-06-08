package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

func (h Handlers) emitAudit(r *http.Request, actorUserID, action, resourceType, resourceID, projectID, outcome, errorCode string, details map[string]any) {
	if h.auditStore == nil {
		return
	}
	event := audit.New(
		strings.TrimSpace(actorUserID),
		requestIP(r),
		requestUserAgent(r),
		strings.TrimSpace(action),
		strings.TrimSpace(resourceType),
		strings.TrimSpace(resourceID),
		strings.TrimSpace(projectID),
		strings.TrimSpace(outcome),
		strings.TrimSpace(errorCode),
		details,
	)
	_ = h.auditStore.Create(event)
}

func (h Handlers) emitAdvancedAudit(r *http.Request, actorUserID, action, resourceType, resourceID, projectID, outcome, errorCode string, details map[string]any) {
	if !h.featureEnabled(edition.FeatureAdvancedAudit) {
		return
	}
	h.emitAudit(r, actorUserID, action, resourceType, resourceID, projectID, outcome, errorCode, details)
}

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func requestUserAgent(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.UserAgent())
}

func parseRFC3339Param(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	u := t.UTC()
	return &u, nil
}

func (h Handlers) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdminModuleFromSessionOrBearer(w, r, "audit")
	if !ok {
		return
	}
	if h.auditStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "audit store is not configured"})
		return
	}

	since, err := parseRFC3339Param(r.URL.Query().Get("since"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid since (RFC3339 expected)"})
		return
	}
	until, err := parseRFC3339Param(r.URL.Query().Get("until"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid until (RFC3339 expected)"})
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		var parsed int
		_, _ = fmt.Sscanf(raw, "%d", &parsed)
		if parsed > 0 {
			limit = parsed
		}
	}
	items, err := h.auditStore.List(store.AuditFilter{
		Since:       since,
		Until:       until,
		Action:      strings.TrimSpace(r.URL.Query().Get("action")),
		ActorUserID: strings.TrimSpace(r.URL.Query().Get("actorUserId")),
		ProjectID:   strings.TrimSpace(r.URL.Query().Get("projectId")),
		Limit:       limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list audit events"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) ExportAuditCSV(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdminModuleFromSessionOrBearer(w, r, "audit")
	if !ok {
		return
	}
	if h.auditStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "audit store is not configured"})
		return
	}
	items, err := h.auditStore.List(store.AuditFilter{Limit: 10000})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to export audit events"})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-audit.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"occurred_at", "actor_user_id", "actor_ip", "action", "resource_type", "resource_id", "project_id", "outcome", "error_code", "details"})
	for _, item := range items {
		details, _ := json.Marshal(item.Details)
		_ = writer.Write([]string{item.OccurredAt.Format(time.RFC3339), item.ActorUserID, item.ActorIP, item.Action, item.ResourceType, item.ResourceID, item.ProjectID, item.Outcome, item.ErrorCode, string(details)})
	}
	writer.Flush()
}
