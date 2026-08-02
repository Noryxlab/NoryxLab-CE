package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/storageendpoint"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type storageEndpointRequest struct {
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	Classification string `json:"classification"`
	Purpose        string `json:"purpose"`
	UseSSL         *bool  `json:"useSSL"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
	DefaultBackup  bool   `json:"defaultBackup"`
	DefaultDataset bool   `json:"defaultDataset"`
}

type storageEndpointTestResult struct {
	OK        bool      `json:"ok"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checkedAt"`
	Message   string    `json:"message,omitempty"`
}

func (h Handlers) ListAdminStorageEndpoints(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "storage"); !ok {
		return
	}
	if h.storageEndpointStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []storageendpoint.Endpoint{}})
		return
	}
	items, err := h.storageEndpointStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list storage endpoints: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateAdminStorageEndpoint(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "storage")
	if !ok {
		return
	}
	if h.storageEndpointStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "storage endpoint store is not configured"})
		return
	}
	var req storageEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	useSSL := true
	if req.UseSSL != nil {
		useSSL = *req.UseSSL
	}
	item := storageendpoint.New(identity.UserID(), req.Name, req.Provider, req.Endpoint, req.Region, req.Classification, req.Purpose, useSSL)
	item.DefaultBackup = req.DefaultBackup
	item.DefaultDataset = req.DefaultDataset
	if msg := validateStorageEndpoint(item); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if strings.TrimSpace(req.AccessKey) == "" || strings.TrimSpace(req.SecretKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "accessKey and secretKey are required"})
		return
	}
	if err := h.saveStorageEndpointSecret(item, req.AccessKey, req.SecretKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save endpoint secret: " + err.Error()})
		return
	}
	if err := h.clearStorageEndpointDefaults(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update endpoint defaults: " + err.Error()})
		return
	}
	if err := h.storageEndpointStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create storage endpoint: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "storage_endpoint.create", "storage_endpoint", item.ID, "", "success", "", map[string]any{"name": item.Name, "classification": item.Classification, "purpose": item.Purpose})
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) UpdateAdminStorageEndpoint(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "storage")
	if !ok {
		return
	}
	endpointID := strings.TrimSpace(r.PathValue("endpointID"))
	item, found, err := h.storageEndpointStore.GetByID(endpointID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load storage endpoint: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "storage endpoint not found"})
		return
	}
	var req storageEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Provider) != "" {
		item.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	}
	if strings.TrimSpace(req.Endpoint) != "" {
		item.EndpointURL = strings.TrimSpace(req.Endpoint)
	}
	if strings.TrimSpace(req.Region) != "" {
		item.Region = strings.TrimSpace(req.Region)
	}
	if strings.TrimSpace(req.Classification) != "" {
		item.Classification = strings.ToLower(strings.TrimSpace(req.Classification))
	}
	if strings.TrimSpace(req.Purpose) != "" {
		item.Purpose = strings.ToLower(strings.TrimSpace(req.Purpose))
	}
	if req.UseSSL != nil {
		item.UseSSL = *req.UseSSL
	}
	item.DefaultBackup = req.DefaultBackup
	item.DefaultDataset = req.DefaultDataset
	item.UpdatedAt = time.Now().UTC()
	if msg := validateStorageEndpoint(item); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if strings.TrimSpace(req.AccessKey) != "" || strings.TrimSpace(req.SecretKey) != "" {
		if strings.TrimSpace(req.AccessKey) == "" || strings.TrimSpace(req.SecretKey) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "both accessKey and secretKey are required when rotating credentials"})
			return
		}
		if err := h.saveStorageEndpointSecret(item, req.AccessKey, req.SecretKey); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save endpoint secret: " + err.Error()})
			return
		}
	}
	if err := h.clearStorageEndpointDefaults(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update endpoint defaults: " + err.Error()})
		return
	}
	if err := h.storageEndpointStore.Update(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update storage endpoint: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "storage_endpoint.update", "storage_endpoint", item.ID, "", "success", "", map[string]any{"name": item.Name})
	writeJSON(w, http.StatusOK, item)
}

func (h Handlers) DeleteAdminStorageEndpoint(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "storage")
	if !ok {
		return
	}
	endpointID := strings.TrimSpace(r.PathValue("endpointID"))
	item, found, err := h.storageEndpointStore.GetByID(endpointID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load storage endpoint: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "storage endpoint not found"})
		return
	}
	if item.DefaultBackup || item.DefaultDataset {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot delete a default storage endpoint"})
		return
	}
	if err := h.storageEndpointStore.Delete(endpointID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete storage endpoint: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "storage_endpoint.delete", "storage_endpoint", item.ID, "", "success", "", map[string]any{"name": item.Name})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h Handlers) TestAdminStorageEndpoint(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "storage")
	if !ok {
		return
	}
	endpointID := strings.TrimSpace(r.PathValue("endpointID"))
	item, found, err := h.storageEndpointStore.GetByID(endpointID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load storage endpoint: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "storage endpoint not found"})
		return
	}
	result := h.testStorageEndpoint(r.Context(), item)
	item.Status = result.Status
	item.StatusMessage = result.Message
	item.LastCheckedAt = result.CheckedAt
	item.UpdatedAt = time.Now().UTC()
	_ = h.storageEndpointStore.Update(item)
	outcome := "success"
	if !result.OK {
		outcome = "failure"
	}
	h.emitAudit(r, identity.UserID(), "storage_endpoint.test", "storage_endpoint", item.ID, "", outcome, result.Message, map[string]any{"name": item.Name})
	writeJSON(w, http.StatusOK, result)
}

func validateStorageEndpoint(item storageendpoint.Endpoint) string {
	if strings.TrimSpace(item.Name) == "" {
		return "name is required"
	}
	parsed, err := url.Parse(strings.TrimSpace(item.EndpointURL))
	if err != nil || parsed.Host == "" || parsed.Scheme == "" {
		return "endpoint must be a valid URL"
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "endpoint scheme must be http or https"
	}
	if item.Classification == "hds" && parsed.Scheme != "https" {
		return "HDS storage endpoints must use HTTPS"
	}
	if item.Provider == "" {
		return "provider is required"
	}
	return ""
}

func (h Handlers) saveStorageEndpointSecret(item storageendpoint.Endpoint, accessKey, secretKey string) error {
	secretStore, ok := h.runtime.(noryxruntime.ControlSecretStore)
	if !ok || secretStore == nil {
		return errControlSecretStoreUnavailable{}
	}
	return secretStore.UpsertControlSecret(noryxruntime.SecretSpec{
		Name: item.SecretName,
		Data: map[string]string{
			"accessKey": strings.TrimSpace(accessKey),
			"secretKey": strings.TrimSpace(secretKey),
			"updatedAt": time.Now().UTC().Format(time.RFC3339),
		},
		Labels: map[string]string{"app.kubernetes.io/name": "noryx-storage-endpoint", "noryx.io/managed-by": "noryx-backend"},
	})
}

func (h Handlers) clearStorageEndpointDefaults(item storageendpoint.Endpoint) error {
	if h.storageEndpointStore == nil || (!item.DefaultBackup && !item.DefaultDataset) {
		return nil
	}
	items, err := h.storageEndpointStore.List()
	if err != nil {
		return err
	}
	for _, current := range items {
		changed := false
		if current.ID != item.ID && item.DefaultBackup && current.DefaultBackup {
			current.DefaultBackup = false
			changed = true
		}
		if current.ID != item.ID && item.DefaultDataset && current.DefaultDataset {
			current.DefaultDataset = false
			changed = true
		}
		if changed {
			current.UpdatedAt = time.Now().UTC()
			if err := h.storageEndpointStore.Update(current); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h Handlers) testStorageEndpoint(ctx context.Context, item storageendpoint.Endpoint) storageEndpointTestResult {
	checkedAt := time.Now().UTC()
	client, err := h.storageEndpointClient(item)
	if err != nil {
		return storageEndpointTestResult{OK: false, Status: "failed", CheckedAt: checkedAt, Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = client.ListBuckets(ctx)
	if err != nil {
		return storageEndpointTestResult{OK: false, Status: "failed", CheckedAt: checkedAt, Message: err.Error()}
	}
	return storageEndpointTestResult{OK: true, Status: "available", CheckedAt: checkedAt}
}

func (h Handlers) storageEndpointClient(item storageendpoint.Endpoint) (*minio.Client, error) {
	secretStore, ok := h.runtime.(noryxruntime.ControlSecretStore)
	if !ok || secretStore == nil {
		return nil, errControlSecretStoreUnavailable{}
	}
	data, found, err := secretStore.GetControlSecret(item.SecretName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errStorageEndpointSecretMissing{}
	}
	parsed, err := url.Parse(strings.TrimSpace(item.EndpointURL))
	if err != nil {
		return nil, err
	}
	return minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(data["accessKey"]), strings.TrimSpace(data["secretKey"]), ""),
		Secure: strings.EqualFold(parsed.Scheme, "https"),
		Region: strings.TrimSpace(item.Region),
	})
}

type errControlSecretStoreUnavailable struct{}

func (errControlSecretStoreUnavailable) Error() string {
	return "kubernetes control secret store is not available"
}

type errStorageEndpointSecretMissing struct{}

func (errStorageEndpointSecretMissing) Error() string { return "storage endpoint secret is missing" }
