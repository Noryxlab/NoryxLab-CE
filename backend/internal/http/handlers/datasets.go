package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type createDatasetRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Bucket         string `json:"bucket"`
	Prefix         string `json:"prefix"`
	Provider       string `json:"provider"`
	Classification string `json:"classification"`
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
}

type datasetS3Credential struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type datasetObjectItem struct {
	Path         string    `json:"path"`
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
	ContentType  string    `json:"contentType,omitempty"`
}

type setDatasetAccessRequest struct {
	Role string `json:"role"`
}
type setDatasetOwnerRequest struct {
	OwnerType string `json:"ownerType"`
	OwnerID   string `json:"ownerId"`
}
type downloadDatasetObjectsRequest struct {
	Paths []string `json:"paths"`
}

type downloadDatasetObjectURLRequest struct {
	Path string `json:"path"`
}

type createDatasetFolderRequest struct {
	Path string `json:"path"`
}

func datasetObjectKey(item dataset.Dataset, objectPath string) (string, string) {
	rel := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(objectPath)), "/")
	key := rel
	if item.Prefix != "" {
		key = strings.Trim(item.Prefix, "/") + "/" + rel
	}
	return rel, key
}

func (h Handlers) datasetRole(item dataset.Dataset, identity auth.Identity) string {
	if item.OwnerType == "" {
		item.OwnerType = "user"
		item.OwnerID = item.OwnerUserID
	}
	best := ""
	for _, subject := range h.datasetSubjects(identity) {
		if strings.EqualFold(item.OwnerType, subject.Type) && strings.EqualFold(item.OwnerID, subject.ID) {
			return "owner"
		}
		access, found, err := h.datasetStore.GetAccess(item.ID, subject.Type, subject.ID)
		if err == nil && found && (access.Role == "writer" || best == "") {
			best = access.Role
		}
	}
	return best
}

func (h Handlers) datasetSubjects(identity auth.Identity) []dataset.Subject {
	subjects := []dataset.Subject{{Type: "user", ID: identity.UserID()}}
	if h.keycloak == nil {
		return subjects
	}
	identifier := strings.TrimSpace(identity.Subject)
	if identifier == "" {
		identifier = identity.UserID()
	}
	organizations, err := h.keycloak.ListUserOrganizations(identifier)
	if err != nil {
		return subjects
	}
	for _, organization := range organizations {
		subjects = append(subjects, dataset.Subject{Type: "organization", ID: organization.ID})
	}
	return subjects
}

func (h Handlers) datasetAvailableInEdition(item dataset.Dataset) bool {
	return item.Classification != "hds" || h.featureEnabled(edition.FeatureHDSDatasets)
}

func (h Handlers) featureEnabled(feature string) bool {
	return h.editionHooks.Feature != nil && h.editionHooks.Feature.Enabled(feature)
}

func (h Handlers) filterDatasetsForEdition(items []dataset.Dataset) []dataset.Dataset {
	out := make([]dataset.Dataset, 0, len(items))
	for _, item := range items {
		if h.datasetAvailableInEdition(item) {
			out = append(out, item)
		}
	}
	return out
}

func (h Handlers) canReadDataset(item dataset.Dataset, identity auth.Identity) bool {
	return h.datasetAvailableInEdition(item) && (h.isGlobalAdmin(identity) || h.datasetRole(item, identity) != "")
}

func (h Handlers) canWriteDataset(item dataset.Dataset, identity auth.Identity) bool {
	if !h.datasetAvailableInEdition(item) {
		return false
	}
	role := h.datasetRole(item, identity)
	return h.isGlobalAdmin(identity) || role == "owner" || role == "writer"
}

func (h Handlers) canManageDatasetAccess(item dataset.Dataset, identity auth.Identity) bool {
	if !h.datasetAvailableInEdition(item) {
		return false
	}
	if item.Classification == "hds" {
		return h.isGlobalAdmin(identity)
	}
	return h.isGlobalAdmin(identity) || h.datasetRole(item, identity) == "owner"
}

var bucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func (h Handlers) ListDatasets(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	items, err := h.datasetStore.ListBySubjects(h.datasetSubjects(identity))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasets"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": h.filterDatasetsForEdition(items)})
}

func (h Handlers) CreateDataset(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
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
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.Classification = strings.ToLower(strings.TrimSpace(req.Classification))
	if req.Provider == "" {
		req.Provider = "minio"
	}
	if req.Provider != "minio" && req.Provider != "s3" && req.Provider != "clever-cloud" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider must be minio, s3, or clever-cloud"})
		return
	}
	if req.Classification != "hds" {
		req.Classification = "non-hds"
	}
	if req.Classification == "hds" && !h.featureEnabled(edition.FeatureHDSDatasets) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "HDS dataset management requires NoryxLab Enterprise Edition"})
		return
	}
	if req.Classification == "hds" && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required to register an HDS dataset"})
		return
	}
	if req.Provider == "minio" && req.Classification == "hds" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "HDS datasets must use a dedicated external S3 connection"})
		return
	}
	if req.Provider != "minio" {
		if strings.TrimSpace(req.Bucket) == "" || strings.TrimSpace(req.Endpoint) == "" || strings.TrimSpace(req.AccessKey) == "" || strings.TrimSpace(req.SecretKey) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint, bucket, accessKey, and secretKey are required for external S3 datasets"})
			return
		}
		if strings.TrimSpace(h.secretsMasterKey) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "secrets encryption key is not configured"})
			return
		}
	}
	item := dataset.New(userID, req.Name, req.Description, req.Bucket, req.Prefix, req.Provider, req.Classification, req.Endpoint, req.Region)
	if item.Bucket == "" {
		item.Bucket = "noryx-ds-" + sanitizeK8sName(item.ID)
	}
	if !bucketNamePattern.MatchString(item.Bucket) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid bucket name"})
		return
	}
	client := h.minioClient
	region := h.minioRegion
	var err error
	var credentialItem secret.Secret
	if item.Provider != "minio" {
		client, err = newDedicatedDatasetS3Client(item.Endpoint, item.Region, req.AccessKey, req.SecretKey)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		region = item.Region
	}
	if client != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		exists, err := client.BucketExists(ctx, item.Bucket)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset bucket check failed: " + err.Error()})
			return
		}
		if !exists && item.Provider == "minio" {
			err = client.MakeBucket(ctx, item.Bucket, minio.MakeBucketOptions{Region: region})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset bucket creation failed: " + err.Error()})
				return
			}
		} else if !exists {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "external S3 bucket does not exist or is not accessible with the configured service profile"})
			return
		}
	}
	if item.Provider != "minio" {
		credentialPayload, err := json.Marshal(datasetS3Credential{AccessKey: req.AccessKey, SecretKey: req.SecretKey})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to prepare S3 credentials"})
			return
		}
		encrypted, err := security.EncryptString(h.secretsMasterKey, string(credentialPayload))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt S3 credentials"})
			return
		}
		item.CredentialName = "dataset-s3-" + item.ID
		credentialItem = secret.New(userID, item.CredentialName, "dataset-s3", encrypted)
		if err := h.secretStore.Upsert(credentialItem); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store encrypted S3 credentials"})
			return
		}
	}
	if err := h.datasetStore.Create(item); err != nil {
		if item.CredentialName != "" {
			_ = h.secretStore.Delete(userID, item.CredentialName)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create dataset"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.create", "dataset", item.ID, "", "success", "", map[string]any{
		"name": item.Name, "provider": item.Provider, "classification": item.Classification, "bucket": item.Bucket,
	})
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) PutDatasetObject(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
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
	if !found || !h.canWriteDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	fullKey := objectPath
	if item.Prefix != "" {
		fullKey = strings.Trim(item.Prefix, "/") + "/" + objectPath
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 512*1024*1024))
	if err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "failure", "payload_read_failed", datasetTransferAuditDetails(item, objectPath, 0))
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read payload"})
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	_, err = client.PutObject(ctx, item.Bucket, fullKey, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "failure", "s3_upload_failed", datasetTransferAuditDetails(item, objectPath, int64(len(payload))))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset upload failed: " + err.Error()})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "success", "", datasetTransferAuditDetails(item, objectPath, int64(len(payload))))
	writeJSON(w, http.StatusCreated, map[string]any{"bucket": item.Bucket, "key": fullKey, "size": len(payload)})
}

func (h Handlers) CreateDatasetFolder(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canWriteDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	var req createDatasetFolderRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	rel := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(req.Path)), "/")
	if rel == "" || rel == "." {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "folder path is required"})
		return
	}
	key := rel + "/"
	if item.Prefix != "" {
		key = strings.Trim(item.Prefix, "/") + "/" + key
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if _, err := client.PutObject(ctx, item.Bucket, key, bytes.NewReader(nil), 0, minio.PutObjectOptions{ContentType: "application/x-directory"}); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset folder creation failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": rel + "/"})
}

func (h Handlers) DeleteDatasetObject(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canWriteDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	rel, key := datasetObjectKey(item, r.PathValue("path"))
	if rel == "" || rel == "." {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path is required"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if r.URL.Query().Get("recursive") != "true" {
		if err := client.RemoveObject(ctx, item.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset object deletion failed: " + err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	prefix := strings.TrimSuffix(key, "/") + "/"
	objects := make(chan minio.ObjectInfo)
	go func() {
		defer close(objects)
		for object := range client.ListObjects(ctx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err == nil {
				objects <- object
			}
		}
	}()
	for removeErr := range client.RemoveObjects(ctx, item.Bucket, objects, minio.RemoveObjectsOptions{}) {
		if removeErr.Err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset folder deletion failed: " + removeErr.Err.Error()})
			return
		}
	}
	_ = client.RemoveObject(ctx, item.Bucket, prefix, minio.RemoveObjectOptions{})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) ListDatasetObjects(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	if datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetID is required"})
		return
	}
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
		return
	}

	prefix := strings.Trim(item.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	objects := []datasetObjectItem{}
	for obj := range client.ListObjects(ctx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset object listing failed: " + obj.Err.Error()})
			return
		}
		relPath := obj.Key
		if prefix != "" && strings.HasPrefix(relPath, prefix) {
			relPath = strings.TrimPrefix(relPath, prefix)
		}
		if relPath == "" {
			continue
		}
		objects = append(objects, datasetObjectItem{
			Path:         relPath,
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ContentType:  obj.ContentType,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": objects})
}

func (h Handlers) GetDatasetObject(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if item.Classification == "hds" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "direct HDS dataset download is disabled"})
		return
	}
	rel, key := datasetObjectKey(item, r.PathValue("path"))
	if rel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path is required"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": datasetS3Error(err)})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	obj, err := client.GetObject(ctx, item.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset download failed"})
		return
	}
	defer obj.Close()
	info, err := obj.Stat()
	if err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.download", "dataset", item.ID, "", "failure", "object_not_found", datasetTransferAuditDetails(item, rel, 0))
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset object not found"})
		return
	}
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(filepath.Base(rel), `"`, "")+`"`)
	written, copyErr := io.Copy(w, obj)
	if copyErr != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.download", "dataset", item.ID, "", "failure", "stream_interrupted", datasetTransferAuditDetails(item, rel, written))
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.download", "dataset", item.ID, "", "success", "", datasetTransferAuditDetails(item, rel, written))
}

func (h Handlers) DownloadDatasetObjects(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if item.Classification == "hds" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "HDS dataset ZIP download is disabled"})
		return
	}
	var req downloadDatasetObjectsRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Paths) == 0 || len(req.Paths) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "between 1 and 100 paths are required"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": datasetS3Error(err)})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeK8sName(item.Name)+`-files.zip"`)
	zw := zip.NewWriter(w)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	var downloadedBytes int64
	downloadedObjects := 0
	failedObjects := 0
	for _, requested := range req.Paths {
		rel, key := datasetObjectKey(item, requested)
		if rel == "" {
			continue
		}
		obj, err := client.GetObject(ctx, item.Bucket, key, minio.GetObjectOptions{})
		if err != nil {
			failedObjects++
			continue
		}
		entry, err := zw.Create(rel)
		if err == nil {
			written, copyErr := io.Copy(entry, obj)
			downloadedBytes += written
			if copyErr == nil {
				downloadedObjects++
			} else {
				failedObjects++
			}
		} else {
			failedObjects++
		}
		obj.Close()
	}
	closeErr := zw.Close()
	outcome := "success"
	errorCode := ""
	if failedObjects > 0 || closeErr != nil {
		outcome = "failure"
		errorCode = "zip_partial_failure"
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.objects.download_zip", "dataset", item.ID, "", outcome, errorCode, map[string]any{
		"datasetName":     item.Name,
		"provider":        item.Provider,
		"classification":  item.Classification,
		"requestedCount":  len(req.Paths),
		"downloadedCount": downloadedObjects,
		"failedCount":     failedObjects,
		"bytes":           downloadedBytes,
	})
}

func (h Handlers) CreateDatasetObjectDownloadURL(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if item.Classification == "hds" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "direct HDS dataset download is disabled"})
		return
	}
	var req downloadDatasetObjectURLRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path is required"})
		return
	}
	rel, key := datasetObjectKey(item, req.Path)
	if rel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path is required"})
		return
	}
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": datasetS3Error(err)})
		return
	}
	expiry := 15 * time.Minute
	params := url.Values{}
	params.Set("response-content-disposition", `attachment; filename="`+strings.ReplaceAll(filepath.Base(rel), `"`, "")+`"`)
	presignedURL, err := client.PresignedGetObject(r.Context(), item.Bucket, key, expiry, params)
	if err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.download_authorize", "dataset", item.ID, "", "failure", "presign_failed", datasetTransferAuditDetails(item, rel, 0))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to prepare dataset download"})
		return
	}
	details := datasetTransferAuditDetails(item, rel, 0)
	details["expiresInSeconds"] = int(expiry.Seconds())
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.download_authorize", "dataset", item.ID, "", "success", "", details)
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       presignedURL.String(),
		"filename":  filepath.Base(rel),
		"expiresAt": time.Now().UTC().Add(expiry),
	})
}

func datasetTransferAuditDetails(item dataset.Dataset, objectPath string, size int64) map[string]any {
	return map[string]any{
		"datasetName":    item.Name,
		"objectPath":     objectPath,
		"provider":       item.Provider,
		"classification": item.Classification,
		"bytes":          size,
	}
}

func (h Handlers) ListDatasetAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || !h.canReadDataset(item, identity) {
		writeJSON(w, 404, map[string]string{"error": "dataset not found"})
		return
	}
	items, err := h.datasetStore.ListAccess(item.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to list dataset permissions"})
		return
	}
	owner := dataset.Access{DatasetID: item.ID, UserID: item.OwnerID, SubjectType: item.OwnerType, SubjectID: item.OwnerID, Role: "owner", CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	writeJSON(w, 200, map[string]any{"items": append([]dataset.Access{owner}, items...), "canManage": h.canManageDatasetAccess(item, identity)})
}

func (h Handlers) SetDatasetAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found {
		writeJSON(w, 404, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canManageDatasetAccess(item, identity) {
		writeJSON(w, 403, map[string]string{"error": "dataset owner or global admin required"})
		return
	}
	subjectType := strings.TrimSpace(r.PathValue("subjectType"))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	if subjectType == "" {
		subjectType = "user"
		subjectID = strings.TrimSpace(r.PathValue("userID"))
	}
	var req setDatasetAccessRequest
	if (subjectType != "user" && subjectType != "organization") || subjectID == "" || json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "valid subjectType, subjectID, and role are required"})
		return
	}
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "reader" && req.Role != "writer" {
		writeJSON(w, 400, map[string]string{"error": "role must be reader or writer"})
		return
	}
	if subjectType == "organization" && !h.organizationExists(subjectID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization does not exist"})
		return
	}
	if strings.EqualFold(subjectType, item.OwnerType) && strings.EqualFold(subjectID, item.OwnerID) {
		writeJSON(w, 400, map[string]string{"error": "owner role cannot be changed"})
		return
	}
	now := time.Now().UTC()
	access := dataset.Access{DatasetID: item.ID, UserID: subjectID, SubjectType: subjectType, SubjectID: subjectID, Role: req.Role, CreatedAt: now, UpdatedAt: now}
	if existing, exists, _ := h.datasetStore.GetAccess(item.ID, subjectType, subjectID); exists {
		access.CreatedAt = existing.CreatedAt
	}
	if err := h.datasetStore.SetAccess(access); err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to set dataset permission"})
		return
	}
	writeJSON(w, 200, access)
}

func (h Handlers) DeleteDatasetAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found {
		writeJSON(w, 404, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canManageDatasetAccess(item, identity) {
		writeJSON(w, 403, map[string]string{"error": "dataset owner or global admin required"})
		return
	}
	subjectType := strings.TrimSpace(r.PathValue("subjectType"))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	if subjectType == "" {
		subjectType = "user"
		subjectID = strings.TrimSpace(r.PathValue("userID"))
	}
	if strings.EqualFold(subjectType, item.OwnerType) && strings.EqualFold(subjectID, item.OwnerID) {
		writeJSON(w, 400, map[string]string{"error": "owner permission cannot be removed"})
		return
	}
	if err := h.datasetStore.DeleteAccess(item.ID, subjectType, subjectID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to delete dataset permission"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) UpdateDatasetOwner(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canManageDatasetAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "dataset owner or global admin required"})
		return
	}
	var req setDatasetOwnerRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ownerType and ownerId are required"})
		return
	}
	req.OwnerType = strings.ToLower(strings.TrimSpace(req.OwnerType))
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	if (req.OwnerType != "user" && req.OwnerType != "organization") || req.OwnerID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownerType must be user or organization and ownerId is required"})
		return
	}
	if req.OwnerType == "organization" && !h.organizationExists(req.OwnerID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization does not exist"})
		return
	}
	if req.OwnerType == "organization" && !h.isGlobalAdmin(identity) {
		isMember := false
		for _, subject := range h.datasetSubjects(identity) {
			if subject.Type == "organization" && subject.ID == req.OwnerID {
				isMember = true
				break
			}
		}
		if !isMember {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "destination organization membership or global admin required"})
			return
		}
	}
	if err := h.datasetStore.UpdateOwner(item.ID, req.OwnerType, req.OwnerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update dataset owner"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.owner.transfer", "dataset", item.ID, "", "success", "", map[string]any{"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID, "ownerType": req.OwnerType, "ownerId": req.OwnerID})
	updated, _, _ := h.datasetStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

func (h Handlers) organizationExists(organizationID string) bool {
	if h.keycloak == nil {
		return false
	}
	organizations, err := h.keycloak.ListOrganizations()
	if err != nil {
		return false
	}
	for _, organization := range organizations {
		if organization.ID == strings.TrimSpace(organizationID) && organization.Enabled {
			return true
		}
	}
	return false
}

func (h Handlers) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	if datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetID is required"})
		return
	}
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found || !h.datasetAvailableInEdition(item) || (!h.isGlobalAdmin(identity) && h.datasetRole(item, identity) != "owner") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if item.Classification == "hds" && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required to delete an HDS dataset"})
		return
	}
	if h.runtime != nil && strings.TrimSpace(item.Bucket) != "" {
		volumeName := "dataset-" + sanitizeK8sName(item.ID)
		if err := h.runtime.DeleteS3Volume(volumeName); err != nil && !isNotFoundError(err) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to delete direct S3 dataset mount: " + err.Error()})
			return
		}
	}
	if item.Provider == "minio" && h.minioClient != nil && strings.TrimSpace(item.Bucket) != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		objects := h.minioClient.ListObjects(ctx, item.Bucket, minio.ListObjectsOptions{Recursive: true})
		for removeErr := range h.minioClient.RemoveObjects(ctx, item.Bucket, objects, minio.RemoveObjectsOptions{}) {
			if removeErr.Err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to delete local dataset objects: " + removeErr.Err.Error()})
				return
			}
		}
		if err := h.minioClient.RemoveBucket(ctx, item.Bucket); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to delete local dataset bucket: " + err.Error()})
			return
		}
	}
	if err := h.datasetStore.Delete(datasetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete dataset"})
		return
	}
	if item.CredentialName != "" {
		credentialUserID := item.CredentialUserID
		if credentialUserID == "" {
			credentialUserID = item.OwnerUserID
		}
		_ = h.secretStore.Delete(credentialUserID, item.CredentialName)
	}
	w.WriteHeader(http.StatusNoContent)
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
			if h.datasetAvailableInEdition(item) {
				items = append(items, item)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AttachProjectDataset(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	if projectID == "" || datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasetID are required"})
		return
	}
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canAssignDataset(identity, item) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": datasetAssignmentError(item)})
		return
	}
	if exists, err := h.projectExists(projectID); err != nil || !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if err := h.projectResourceStore.AttachDataset(projectID, datasetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach dataset"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) DetachProjectDataset(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	if projectID == "" || datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasetID are required"})
		return
	}
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canAssignDataset(identity, item) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": datasetAssignmentError(item)})
		return
	}
	if err := h.projectResourceStore.DetachDataset(projectID, datasetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach dataset"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) canAssignDataset(identity auth.Identity, item dataset.Dataset) bool {
	if !h.datasetAvailableInEdition(item) {
		return false
	}
	if h.isGlobalAdmin(identity) {
		return true
	}
	return item.Classification != "hds" && h.datasetRole(item, identity) == "owner"
}

func datasetAssignmentError(item dataset.Dataset) string {
	if item.Classification == "hds" {
		return "HDS dataset management requires NoryxLab Enterprise Edition"
	}
	return "dataset owner or global admin role required to assign this dataset"
}

func (h Handlers) datasetS3Client(item dataset.Dataset) (*minio.Client, string, error) {
	if !h.datasetAvailableInEdition(item) {
		return nil, "", errors.New("HDS dataset management requires NoryxLab Enterprise Edition")
	}
	if item.Provider == "minio" {
		if item.Classification == "hds" {
			return nil, "", errors.New("HDS datasets cannot use the internal MinIO profile")
		}
		return h.minioClient, h.minioRegion, nil
	}
	if item.CredentialName != "" {
		credentialUserID := item.CredentialUserID
		if credentialUserID == "" {
			credentialUserID = item.OwnerUserID
		}
		credentialItem, found, err := h.secretStore.GetByName(credentialUserID, item.CredentialName)
		if err != nil {
			return nil, "", errors.New("failed to read dataset S3 credentials")
		}
		if !found {
			return nil, "", errors.New("dataset S3 credentials are missing")
		}
		decrypted, err := security.DecryptString(h.secretsMasterKey, credentialItem.ValueEncrypted)
		if err != nil {
			return nil, "", errors.New("failed to decrypt dataset S3 credentials")
		}
		var credential datasetS3Credential
		if err := json.Unmarshal([]byte(decrypted), &credential); err != nil {
			return nil, "", errors.New("invalid dataset S3 credentials")
		}
		client, err := newDedicatedDatasetS3Client(item.Endpoint, item.Region, credential.AccessKey, credential.SecretKey)
		return client, item.Region, err
	}
	return nil, "", errors.New("dataset-dedicated S3 credentials are not configured")
}

func newDedicatedDatasetS3Client(endpoint, region, accessKey, secretKey string) (*minio.Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.New("external S3 endpoint must be a valid HTTPS URL")
	}
	return minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(accessKey), strings.TrimSpace(secretKey), ""),
		Secure: true,
		Region: strings.TrimSpace(region),
	})
}

func datasetS3Error(err error) string {
	if err != nil {
		return err.Error()
	}
	return "object storage service profile is not configured"
}
