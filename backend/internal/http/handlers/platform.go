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
	// What this figure covers, and what it leaves out.
	//
	// It read "656 Mo - volume measured on the reachable buckets" while one
	// regulated dataset alone held several gigabytes. Two different silences
	// produced that: regulated datasets are deliberately not enumerated here,
	// and a measurement that runs past its deadline stops early. Neither was
	// visible, so the number looked like a total and was a sample.
	//
	// The counts below are reported so the interface can say which datasets
	// the figure speaks for. A measurement that cannot state its own coverage
	// is a measurement nobody should act on.
	var storageBytes int64
	storageDatasets := 0
	storageRegulated := 0
	storageUnreadable := 0
	storageTruncated := false
	for _, item := range datasets {
		if strings.EqualFold(item.Classification, "hds") {
			// Not enumerated on purpose: listing a regulated bucket means the
			// platform walking the keys of health data to produce a figure on
			// a home page, which is not a trade worth making.
			storageRegulated++
			continue
		}
		client, _, err := h.datasetS3Client(item)
		if err != nil || client == nil {
			storageUnreadable++
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
		} else {
			storageUnreadable++
		}
		if ctx.Err() != nil {
			// The deadline stopped the walk. Whatever has not been visited is
			// not zero, and the interface has to be able to say so.
			storageTruncated = true
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
			// Left out because they are regulated, and left out because they
			// could not be read, are different facts with different remedies.
			"datasetsRegulated":  storageRegulated,
			"datasetsUnreadable": storageUnreadable,
			// True when the deadline cut the walk short, so the figure is a
			// floor rather than a total.
			"truncated": storageTruncated,
		},
	})
}
