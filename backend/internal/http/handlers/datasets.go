package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

type createDatasetRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Bucket      string `json:"bucket"`
	Prefix      string `json:"prefix"`
}

var bucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func (h Handlers) ListDatasets(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.datasetStore.ListByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasets"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateDataset(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req createDatasetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	item := dataset.New(userID, req.Name, req.Description, req.Bucket, req.Prefix)
	if item.Bucket == "" {
		item.Bucket = "noryx-ds-" + sanitizeK8sName(item.ID)
	}
	if !bucketNamePattern.MatchString(item.Bucket) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid bucket name"})
		return
	}
	if h.minioClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		exists, err := h.minioClient.BucketExists(ctx, item.Bucket)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset bucket check failed: " + err.Error()})
			return
		}
		if !exists {
			err = h.minioClient.MakeBucket(ctx, item.Bucket, minio.MakeBucketOptions{Region: h.minioRegion})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset bucket creation failed: " + err.Error()})
				return
			}
		}
	}
	if err := h.datasetStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create dataset"})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) PutDatasetObject(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.minioClient == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "object storage is not configured"})
		return
	}
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	objectPath := strings.TrimSpace(r.PathValue("path"))
	objectPath = strings.TrimPrefix(path.Clean("/"+objectPath), "/")
	if datasetID == "" || objectPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetID and object path are required"})
		return
	}
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	fullKey := objectPath
	if item.Prefix != "" {
		fullKey = strings.Trim(item.Prefix, "/") + "/" + objectPath
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 512*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read payload"})
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	_, err = h.minioClient.PutObject(ctx, item.Bucket, fullKey, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset upload failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"bucket": item.Bucket, "key": fullKey, "size": len(payload)})
}

func (h Handlers) ListProjectDatasets(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectMember(w, projectID, userID, "dataset listing") {
		return
	}
	ids, err := h.projectResourceStore.ListProjectDatasetIDs(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list project datasets"})
		return
	}
	items := make([]dataset.Dataset, 0, len(ids))
	for _, id := range ids {
		item, found, err := h.datasetStore.GetByID(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load project dataset"})
			return
		}
		if found {
			items = append(items, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AttachProjectDataset(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	if projectID == "" || datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasetID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "dataset attach") {
		return
	}
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if err := h.projectResourceStore.AttachDataset(projectID, datasetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach dataset"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) DetachProjectDataset(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	if projectID == "" || datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasetID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "dataset detach") {
		return
	}
	if err := h.projectResourceStore.DetachDataset(projectID, datasetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach dataset"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
