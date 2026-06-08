package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/minio/minio-go/v7"
)

func (h Handlers) GetPlatformOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentity(w, r); !ok {
		return
	}
	projects, projectErr := h.projectStore.List()
	datasets, datasetErr := h.datasetStore.ListAll()
	workspaces, workspaceErr := h.workspaceStore.List()
	apps, appErr := h.appStore.List()
	jobs, jobErr := h.jobStore.List()
	builds, buildErr := h.buildStore.List()
	if projectErr != nil || datasetErr != nil || workspaceErr != nil || appErr != nil || jobErr != nil || buildErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build platform overview"})
		return
	}
	datasets = h.filterDatasetsForEdition(datasets)

	userCount := 0
	if h.keycloak != nil {
		if users, err := h.keycloak.ListUsers(); err == nil {
			userCount = len(users)
		}
	}
	active := 0
	for _, status := range appendExecutionStatuses(workspaces, apps, jobs, builds) {
		if status == "running" || status == "launching" || status == "submitted" {
			active++
		}
	}
	metrics := noryxruntime.WorkloadMetrics{}
	if inspector, ok := h.runtime.(noryxruntime.WorkloadMetricsInspector); ok {
		metrics, _ = inspector.GetWorkloadMetrics()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	var storageBytes int64
	storageDatasets := 0
	for _, item := range datasets {
		if strings.EqualFold(item.Classification, "hds") {
			continue
		}
		client, _, err := h.datasetS3Client(item)
		if err != nil || client == nil {
			continue
		}
		prefix := strings.Trim(item.Prefix, "/")
		if prefix != "" {
			prefix += "/"
		}
		var datasetBytes int64
		readable := true
		for object := range client.ListObjects(ctx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err != nil {
				readable = false
				break
			}
			datasetBytes += object.Size
		}
		if readable {
			storageBytes += datasetBytes
			storageDatasets++
		}
		if ctx.Err() != nil {
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sampledAt": time.Now().UTC(),
		"counts": map[string]int{
			"users":    userCount,
			"projects": len(projects),
			"datasets": len(datasets),
			"active":   active,
		},
		"workloadMetrics": metrics,
		"storage": map[string]any{
			"bytes":            storageBytes,
			"datasetsMeasured": storageDatasets,
			"datasetsTotal":    len(datasets),
		},
	})
}
