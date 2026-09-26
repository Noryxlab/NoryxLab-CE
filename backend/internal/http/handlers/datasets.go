package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
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
	// OwnerID names the organization a regulated dataset belongs to. Required
	// for an HDS classification and ignored otherwise: ordinary datasets
	// belong to whoever registered them, which is the behaviour every
	// installation already has.
	OwnerID string `json:"ownerId"`
}

type updateDatasetMetadataRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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
	// IsPrefix marks a folder. Without it the browser had to infer folders
	// from full keys, which only works when it has been handed every key in
	// the bucket - the thing that made this unusable on a real dataset.
	IsPrefix bool `json:"isPrefix,omitempty"`
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
	for _, teamID := range h.callerTeamIDs(identity) {
		subjects = append(subjects, dataset.Subject{Type: "team", ID: teamID})
	}
	return subjects
}

func (h Handlers) datasetAvailableInEdition(item dataset.Dataset) bool {
	return item.Classification != "hds" || h.hdsDatasetsAvailable()
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
	items, err := h.datasetsVisibleTo(identity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasets"})
		return
	}
	visible := h.filterDatasetsForEdition(items)
	h.nameDatasetOwners(visible)
	writeJSON(w, http.StatusOK, map[string]any{"items": visible})
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
	if req.Provider == "clever-cloud" {
		req.Provider = "s3"
	}
	if req.Provider != "minio" && req.Provider != "s3" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider must be minio or s3"})
		return
	}
	if req.Classification != "hds" {
		req.Classification = "non-hds"
	}
	if req.Classification == "hds" && !h.hdsDatasetsAvailable() {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "HDS dataset management requires NoryxLab Enterprise Edition"})
		return
	}
	if req.Classification == "hds" && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required to register an HDS dataset"})
		return
	}
	if req.Classification == "hds" && strings.TrimSpace(req.OwnerID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "a health dataset belongs to an organization: name one as its owner",
		})
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
	if req.Classification == "hds" {
		organization, found := h.resolveOrganization(strings.TrimSpace(req.OwnerID))
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no organization named " + req.OwnerID})
			return
		}
		item.OwnerType = "organization"
		item.OwnerID = organization.ID
	}
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

func (h Handlers) UpdateDatasetMetadata(w http.ResponseWriter, r *http.Request) {
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
	if !found || !h.datasetAvailableInEdition(item) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canManageDatasetAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "dataset owner or global admin role required to update this dataset"})
		return
	}
	var req updateDatasetMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	description := strings.TrimSpace(req.Description)
	if err := h.datasetStore.UpdateMetadata(datasetID, req.Name, description); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update dataset"})
		return
	}
	item.Name = req.Name
	item.Description = description
	item.UpdatedAt = time.Now().UTC()
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.update", "dataset", item.ID, "", "success", "", map[string]any{
		"name": item.Name,
	})
	writeJSON(w, http.StatusOK, item)
}

// datasetUploadBody prepares the request body to be streamed into storage.
//
// Two things happen here that did not before. Too large is refused rather than
// trimmed: the body used to be read through an io.LimitReader capped at 512 MB,
// which does not fail on a larger one - it stops. A 600 MB study was stored as
// its first 512 MB, answered 200, and went into the audit trail as a success.
// On a regulated dataset that is a corrupted record nobody has any reason to go
// looking for.
//
// And it is streamed rather than buffered: io.ReadAll held the whole object in
// the backend's heap, so three people uploading at once cost three times the
// object in a pod that serves everything else as well. What that produces is
// not a failed upload, it is a platform that disappears.
//
// MaxBytesReader stays on the stream because Content-Length is a claim the
// caller makes, not a fact. A size of -1 tells the storage client to send in
// parts, which is also how a body with no declared length is handled: unknown
// is not a reason to buffer it.
func datasetUploadBody(w http.ResponseWriter, r *http.Request) (io.ReadCloser, int64, bool) {
	if r.ContentLength > maxDatasetObjectBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": fmt.Sprintf("this object is larger than the %d GiB an upload may carry",
				maxDatasetObjectBytes>>30)})
		return nil, 0, false
	}
	size := r.ContentLength
	if size < 0 {
		size = -1
	}
	// The stream is held to the tighter of the two limits: what the caller
	// declared, or the ceiling when they declared nothing. Storage reads
	// exactly the declared count anyway, so this changes no honest upload - it
	// means a caller who announces a kilobyte and sends a gigabyte is stopped
	// at the kilobyte rather than at five gibibytes.
	ceiling := int64(maxDatasetObjectBytes)
	if size > 0 {
		ceiling = size
	}
	return http.MaxBytesReader(w, r.Body, ceiling), size, true
}

// datasetUploadDeadline scales the timeout to what is actually being sent.
//
// A flat two minutes was enough for the uploads the web page makes and cut off
// anything large over an ordinary link, leaving a part-written object and a
// caller who had waited for it. One second per megabyte is roughly 8 Mb/s, slow
// enough to be a genuine timeout rather than a speed limit.
func datasetUploadDeadline(size int64) time.Duration {
	deadline := 2 * time.Minute
	if size > 0 {
		deadline += time.Duration(size/(1<<20)) * time.Second
	}
	return deadline
}

// maxDatasetObjectBytes is the largest single object an upload may carry.
//
// Five gibibytes is the ceiling a single S3 PUT accepts; beyond it a client
// has to compose the object from parts, which is a different conversation than
// a size limit. The previous cap was 512 MB, unstated and enforced by
// truncation.
const maxDatasetObjectBytes = 5 << 30

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
	body, size, ok := datasetUploadBody(w, r)
	if !ok {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "failure", "payload_too_large", datasetTransferAuditDetails(item, objectPath, r.ContentLength))
		return
	}
	defer body.Close()
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ctx, cancel := context.WithTimeout(r.Context(), datasetUploadDeadline(size))
	defer cancel()
	info, err := client.PutObject(ctx, item.Bucket, fullKey, body, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		// A caller who understated Content-Length is stopped by the reader
		// above, and that surfaces here as a failed upload. It is the caller's
		// mistake, not the storage's, so it is worth saying which.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "failure", "payload_too_large", datasetTransferAuditDetails(item, objectPath, size))
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": fmt.Sprintf("this object is larger than the %d GiB an upload may carry",
					maxDatasetObjectBytes>>30)})
			return
		}
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "failure", "s3_upload_failed", datasetTransferAuditDetails(item, objectPath, size))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset upload failed: " + err.Error()})
		return
	}
	// What storage accepted, not what the caller announced: the audit trail of
	// a regulated dataset should record the object that exists.
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.object.upload", "dataset", item.ID, "", "success", "", datasetTransferAuditDetails(item, objectPath, info.Size))
	writeJSON(w, http.StatusCreated, map[string]any{"bucket": item.Bucket, "key": fullKey, "size": info.Size})
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

	// One directory at a time.
	//
	// This listed the whole bucket, recursively, in a single response, and
	// ignored the folder the browser asked for. On a test bucket that is
	// invisible; on HDS-For it is 24,179 objects and 400 GB of metadata behind
	// a 30-second timeout, so the explorer simply never finished - and the
	// folder the user clicked was sent to the *download* route, because the
	// client addressed a listing as `objects/<path>`.
	base := strings.Trim(item.Prefix, "/")
	if base != "" {
		base += "/"
	}
	requested := sanitizeDatasetPrefix(r.URL.Query().Get("prefix"))
	prefix := base + requested

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	objects := []datasetObjectItem{}
	truncated := false
	for obj := range client.ListObjects(ctx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: false}) {
		if obj.Err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset object listing failed: " + obj.Err.Error()})
			return
		}
		relPath := obj.Key
		if base != "" && strings.HasPrefix(relPath, base) {
			relPath = strings.TrimPrefix(relPath, base)
		}
		if relPath == "" || relPath == requested {
			continue
		}
		if len(objects) >= datasetObjectPageSize {
			// Said out loud rather than silently cut: a folder that shows 2,000
			// of its 40,000 files and does not say so is a lie about the data.
			truncated = true
			break
		}
		objects = append(objects, datasetObjectItem{
			Path:         relPath,
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ContentType:  obj.ContentType,
			IsPrefix:     strings.HasSuffix(obj.Key, "/"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": objects, "prefix": requested, "truncated": truncated})
}

// datasetObjectPageSize caps one directory's listing. A folder with more
// entries than this is rare and a browser that tries to render 40,000 rows
// helps nobody.
const datasetObjectPageSize = 2000

// sanitizeDatasetPrefix keeps a request inside the dataset it names: a prefix
// is a folder path under the dataset root, never a way out of it.
func sanitizeDatasetPrefix(raw string) string {
	cleaned := strings.Trim(strings.TrimSpace(raw), "/")
	if cleaned == "" {
		return ""
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == ".." || segment == "." {
			return ""
		}
	}
	return cleaned + "/"
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
	rel, key := datasetObjectKey(item, r.PathValue("path"))
	if rel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "object path is required"})
		return
	}
	if item.Classification == "hds" && !hdsRootDocumentPreviewAllowed(rel) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "direct HDS dataset download is disabled"})
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
		lower := strings.ToLower(rel)
		if strings.HasSuffix(lower, ".pdf") {
			contentType = "application/pdf"
		} else if strings.HasSuffix(lower, ".xlsx") {
			contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		} else if strings.HasSuffix(lower, ".ods") {
			contentType = "application/vnd.oasis.opendocument.spreadsheet"
		} else if strings.HasSuffix(lower, ".csv") {
			contentType = "text/csv; charset=utf-8"
		} else if strings.HasSuffix(lower, ".json") {
			contentType = "application/json; charset=utf-8"
		} else if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".log") || strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
			contentType = "text/plain; charset=utf-8"
		} else if strings.HasSuffix(lower, ".svg") {
			contentType = "image/svg+xml"
		} else if strings.HasSuffix(lower, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
			contentType = "image/jpeg"
		} else if strings.HasSuffix(lower, ".gif") {
			contentType = "image/gif"
		} else if strings.HasSuffix(lower, ".webp") {
			contentType = "image/webp"
		} else {
			contentType = "application/octet-stream"
		}
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

func hdsRootDocumentPreviewAllowed(relPath string) bool {
	rel := strings.Trim(strings.TrimSpace(relPath), "/")
	if rel == "" || strings.Contains(rel, "/") {
		return false
	}
	lower := strings.ToLower(rel)
	return strings.HasSuffix(lower, ".pdf") ||
		strings.HasSuffix(lower, ".xlsx") ||
		strings.HasSuffix(lower, ".ods") ||
		strings.HasSuffix(lower, ".csv") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".log") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".webp") ||
		strings.HasSuffix(lower, ".svg")
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
	all := append([]dataset.Access{owner}, items...)
	h.nameAccessSubjects(all)
	writeJSON(w, 200, map[string]any{"items": all, "canManage": h.canManageDatasetAccess(item, identity)})
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
	if !isGrantableSubjectType(subjectType) || subjectID == "" || json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "valid subjectType, subjectID, and role are required"})
		return
	}
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "reader" && req.Role != "writer" {
		writeJSON(w, 400, map[string]string{"error": "role must be reader or writer"})
		return
	}
	if subjectType == "organization" {
		organization, found := h.resolveOrganization(subjectID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no organization named " + subjectID})
			return
		}
		subjectID = organization.ID
	}
	if subjectType == "team" {
		item, found := h.resolveTeam(subjectID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no team with identifier " + subjectID})
			return
		}
		subjectID = item.ID
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
	// Who may see this, decided by whom, and when.
	//
	// The trail recorded uploads and downloads and not this, which is the
	// wrong half. Reading an object is the consequence; granting the access is
	// the decision, and it is the decision a data protection officer asks
	// about first - "who could see this dataset, since when, and who said so".
	// On 2026-09-21 an entire organisation was found holding read access to a
	// health dataset and nobody could say who had given it, because nothing
	// had been written down. The classification travels with the entry so a
	// regulated dataset can be filtered out of a year of activity without
	// joining anything.
	h.emitAudit(r, identity.UserID(), "dataset.access.grant", "dataset", item.ID, "", "success", "",
		map[string]any{
			"dataset": item.Name, "classification": item.Classification,
			"subjectType": subjectType, "subjectId": subjectID, "role": req.Role,
		})
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
	// A withdrawal is as much of a decision as a grant, and the pair is what
	// makes a period answerable: without it, a dataset nobody can reach today
	// says nothing about who could reach it last month.
	h.emitAudit(r, identity.UserID(), "dataset.access.revoke", "dataset", item.ID, "", "success", "",
		map[string]any{
			"dataset": item.Name, "classification": item.Classification,
			"subjectType": subjectType, "subjectId": subjectID,
		})
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
	if req.OwnerType == "organization" {
		organization, found := h.resolveOrganization(req.OwnerID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no organization named " + req.OwnerID})
			return
		}
		req.OwnerID = organization.ID
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
	// Regulated data belongs to an organization, never to a person.
	//
	// Both were accepted, and the difference was discovered the hard way: a
	// health dataset owned by Essilor was handed to one of its members, who
	// was trying to unblock herself. It did not unblock her - attaching
	// regulated data to a project is a global administrator's decision, which
	// ownership does not confer - and it removed the access every other member
	// of that organization held through it. One well-meant action, two
	// colleagues cut off, and nothing in the way.
	//
	// The day the named person leaves, an organization's health data has an
	// owner who no longer exists. That alone settles it.
	if !ownerAllowedForClassification(item.Classification, req.OwnerType) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": regulatedOwnerError})
		return
	}
	if err := h.datasetStore.UpdateOwner(item.ID, req.OwnerType, req.OwnerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update dataset owner"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.owner.transfer", "dataset", item.ID, "", "success", "", map[string]any{"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID, "ownerType": req.OwnerType, "ownerId": req.OwnerID})
	updated, _, _ := h.datasetStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

const regulatedOwnerError = "a health dataset belongs to an organization, not to a person"

// ownerAllowedForClassification reports whether this kind of owner may hold
// this kind of data.
//
// Regulated data belongs to an organization, never to a person. Both were
// accepted, and the difference was learned from a real one: a health dataset
// owned by Essilor was handed to one of its members who was trying to unblock
// herself. It did not unblock her - attaching regulated data to a project is a
// global administrator's decision, which ownership does not confer - and it
// removed the access every other member of that organization held through it.
// One well-meant action, two colleagues cut off, nothing in the way.
//
// The day the named person leaves, an organization's health data has an owner
// who no longer exists. That alone settles it.
func ownerAllowedForClassification(classification, ownerType string) bool {
	if !strings.EqualFold(strings.TrimSpace(classification), "hds") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(ownerType), "organization")
}

func (h Handlers) organizationExists(organizationID string) bool {
	_, found := h.resolveOrganization(organizationID)
	return found
}

// resolveOrganization accepts whichever handle the caller has - the
// identifier or the alias - and answers with the organization itself.
//
// Keycloak gives an organization both, and the interface offers the alias
// because that is what a person recognises: "imt", not
// 57c801a4-f273-4844-97c6-28307872a480. Every check here compared against the
// identifier alone, so transferring a project to an organization that plainly
// exists was refused with "organization does not exist" - the platform telling
// somebody their own organization is imaginary.
//
// Callers use the resolved ID from here on, so what gets stored is always the
// identifier, whatever the caller typed.
// callerTeamIDs is every team the caller belongs to.
//
// Matched on both the username and the email, for the reason the account list
// already records: a team is composed from whichever identifier the
// administrator had in front of them, and a directory that answers
// "Malinet.C" to a store holding "malinetc" produces an access list where
// half the grants silently do not apply.
func (h Handlers) callerTeamIDs(identity auth.Identity) []string {
	if h.teamStore == nil {
		return nil
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, identifier := range []string{identity.Username, identity.Email, identity.Subject} {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			continue
		}
		items, err := h.teamStore.ListByUser(identifier)
		if err != nil {
			// Degraded rather than denied: a team store that cannot answer
			// must not turn every grant into a refusal.
			log.Printf("access: teams unavailable for %s: %v", identifier, err)
			continue
		}
		for _, item := range items {
			if !seen[item.ID] {
				seen[item.ID] = true
				ids = append(ids, item.ID)
			}
		}
	}
	return ids
}

// resolveTeam accepts a team identifier and answers whether it names one.
//
// By identifier only, never by name: two organizations may each have a team
// called "imagerie", and a grant that resolved a name would hand one
// organization's project to the other one's people.
func (h Handlers) resolveTeam(identifier string) (team.Team, bool) {
	identifier = strings.TrimSpace(identifier)
	if h.teamStore == nil || identifier == "" {
		return team.Team{}, false
	}
	item, found, err := h.teamStore.GetByID(identifier)
	if err != nil || !found {
		return team.Team{}, false
	}
	return item, true
}

func (h Handlers) resolveOrganization(identifier string) (keycloak.Organization, bool) {
	identifier = strings.TrimSpace(identifier)
	if h.keycloak == nil || identifier == "" {
		return keycloak.Organization{}, false
	}
	organizations, err := h.keycloak.ListOrganizations()
	if err != nil {
		return keycloak.Organization{}, false
	}
	for _, organization := range organizations {
		if !organization.Enabled {
			continue
		}
		if organization.ID == identifier || strings.EqualFold(organization.Alias, identifier) {
			return organization, true
		}
	}
	return keycloak.Organization{}, false
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
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachDataset, "dataset attach") {
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
	if !h.canAssignDataset(identity, item, projectID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": h.datasetAssignmentError(item)})
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
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachDataset, "dataset detach") {
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
	if !h.canAssignDataset(identity, item, projectID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": h.datasetAssignmentError(item)})
		return
	}
	if err := h.projectResourceStore.DetachDataset(projectID, datasetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach dataset"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) canAssignDataset(identity auth.Identity, item dataset.Dataset, projectID string) bool {
	if !h.datasetAvailableInEdition(item) {
		return false
	}
	if h.isGlobalAdmin(identity) {
		return true
	}
	if item.Classification == "hds" {
		return h.canAttachRegulatedDataset(identity, item, projectID)
	}
	return h.datasetRole(item, identity) == "owner"
}

// canAttachRegulatedDataset delegates the mount of regulated data to the people
// who already answer for both sides of it.
//
// Attaching an HDS dataset was a global administrator's decision and nobody
// else's. That is defensible on paper and a bottleneck in practice: a team
// whose organisation owns the data, working in a project they administer, had
// to wait on one person for a mount that involved nobody outside their own
// organisation. The wait is not a safeguard; it is a queue, and the safeguard
// it stands in for is entitlement.
//
// So the two halves are asked directly. The dataset must be owned by an
// organisation - a person cannot own regulated data, which registration
// already refuses - and the caller must belong to it: that is the entitlement.
// And the caller must administer the project it is being mounted into, which
// is what makes them answerable for who sees it afterwards. Either half alone
// is not enough, and a global administrator keeps the right they always had.
func (h Handlers) canAttachRegulatedDataset(identity auth.Identity, item dataset.Dataset, projectID string) bool {
	if !entitledToRegulatedDataset(h.datasetSubjects(identity), item) {
		return false
	}
	return h.administersProject(projectID, identity.UserID())
}

// entitledToRegulatedDataset is the entitlement half, on its own.
//
// Separated from the rest because it is the half that can be got wrong
// quietly: it compares an owner recorded by one screen against organisations
// reported by the directory, and both arrive with whatever case and spacing
// their source used. A rule about regulated data must not depend on that.
func entitledToRegulatedDataset(subjects []dataset.Subject, item dataset.Dataset) bool {
	if !strings.EqualFold(strings.TrimSpace(item.OwnerType), "organization") {
		return false
	}
	owner := strings.TrimSpace(item.OwnerID)
	if owner == "" {
		return false
	}
	for _, subject := range subjects {
		if strings.EqualFold(strings.TrimSpace(subject.Type), "organization") &&
			strings.EqualFold(strings.TrimSpace(subject.ID), owner) {
			return true
		}
	}
	return false
}

// administersProject is project administration as the platform decides it:
// the owner, or a role that resolves to administrator. A custom role answers
// through the built-in it is based on, like everywhere else.
func (h Handlers) administersProject(projectID, userID string) bool {
	if item, found, err := h.projectByID(projectID); err == nil && found && h.projectOwnedBy(item, userID) {
		return true
	}
	role, ok := h.effectiveProjectRole(projectID, userID)
	return ok && h.baseRole(role) == access.RoleAdmin
}

// datasetAssignmentError says which of the two refusals actually happened.
//
// It used to answer "requires Enterprise Edition" for every HDS refusal, which
// is true only on Community. On an Enterprise installation the edition is
// present and the real reason is the rule below it: attaching regulated data to
// a project is a global admin's decision. A member reading the old message was
// told their platform lacked a licence it holds, went looking for the wrong
// thing, and had no way to learn that the person who could help was their own
// administrator.
//
// Registration a few hundred lines above already separates these two cases.
// This is the same separation, in the place that was still collapsing them.
func (h Handlers) datasetAssignmentError(item dataset.Dataset) string {
	if !h.datasetAvailableInEdition(item) {
		return "HDS dataset management requires NoryxLab Enterprise Edition"
	}
	if item.Classification == "hds" {
		return "attaching regulated data requires a global admin, or an administrator of this project who belongs to the organisation that owns the dataset"
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
