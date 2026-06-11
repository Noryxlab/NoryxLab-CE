package handlers

import (
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

func (h Handlers) PublishApp(w http.ResponseWriter, r *http.Request) {
	record, userID, ok := h.requireAppOperation(w, r, "app publication")
	if !ok {
		return
	}
	operator, ok := h.runtime.(noryxruntime.PodRevisionOperator)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "app publication is not supported by runtime"})
		return
	}
	manifest, err := operator.GetPodManifest(record.PodName)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "app must be running before publication: " + err.Error()})
		return
	}
	revisions, err := h.appStore.ListRevisions(record.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app revisions"})
		return
	}
	next := 1
	for _, revision := range revisions {
		if revision.Number >= next {
			next = revision.Number + 1
		}
	}
	revision := app.NewRevision(record, next, manifest, userID)
	if err := h.appStore.CreateRevision(revision); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to publish app"})
		return
	}
	h.emitAudit(r, userID, "app.publish", "app", record.ID, record.ProjectID, "success", "", map[string]any{"revision": revision.Number, "name": record.Name})
	writeJSON(w, http.StatusCreated, revision)
}

func (h Handlers) ListAppRevisions(w http.ResponseWriter, r *http.Request) {
	record, _, ok := h.requireAppOperation(w, r, "app revision access")
	if !ok {
		return
	}
	revisions, err := h.appStore.ListRevisions(record.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list app revisions"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": revisions})
}

func (h Handlers) RollbackAppRevision(w http.ResponseWriter, r *http.Request) {
	record, userID, ok := h.requireAppOperation(w, r, "app rollback")
	if !ok {
		return
	}
	revisionID := strings.TrimSpace(r.PathValue("revisionID"))
	revisions, err := h.appStore.ListRevisions(record.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app revisions"})
		return
	}
	var target app.Revision
	found := false
	for _, revision := range revisions {
		if revision.ID == revisionID {
			target = revision
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app revision not found"})
		return
	}
	operator, ok := h.runtime.(noryxruntime.PodRevisionOperator)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "app rollback is not supported by runtime"})
		return
	}
	if err := operator.RestorePodManifest(record.PodName, target.RuntimeManifest); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to restore app revision: " + err.Error()})
		return
	}
	restored := target.Snapshot
	restored.ID = record.ID
	restored.PodName = record.PodName
	restored.ServiceName = record.ServiceName
	restored.Status = "launching"
	restored.Published = true
	restored.ActiveRevision = target.Number
	restored.PublishedAt = &target.PublishedAt
	if err := h.appStore.Upsert(restored); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist restored app revision"})
		return
	}
	if err := h.appStore.ActivateRevision(record.ID, target.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to activate restored app revision"})
		return
	}
	h.emitAudit(r, userID, "app.rollback", "app", record.ID, record.ProjectID, "success", "", map[string]any{"revision": target.Number, "name": record.Name})
	writeJSON(w, http.StatusAccepted, map[string]any{"app": restored, "revision": target})
}

func (h Handlers) ListProductionApps(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.appStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list production apps"})
		return
	}
	filtered := make([]app.App, 0, len(items))
	for _, item := range items {
		if item.Kind == "app" && item.Published && h.hasProjectMembership(userID, item.ProjectID) {
			filtered = append(filtered, h.enrichAppRuntimeStatus(item))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": filtered})
}
