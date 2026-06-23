package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

const backupTargetSecretName = "noryx-backup-target"

type backupTargetStatus struct {
	Configured bool      `json:"configured"`
	Endpoint   string    `json:"endpoint"`
	Bucket     string    `json:"bucket"`
	Prefix     string    `json:"prefix"`
	Region     string    `json:"region"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

type backupTargetRequest struct {
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	Region          string `json:"region"`
	AccessKey       string `json:"accessKey"`
	SecretKey       string `json:"secretKey"`
	EncryptionKeyID string `json:"encryptionKeyId"`
}

func (h Handlers) GetAdminBackupConfigStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireEnterpriseBackup(w) {
		return
	}
	if _, ok := h.requireAdminModule(w, r, "backups"); !ok {
		return
	}
	status, err := h.backupTargetStatus()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read backup target: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h Handlers) UpdateAdminBackupConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireEnterpriseBackup(w) {
		return
	}
	identity, ok := h.requireAdminModule(w, r, "backups")
	if !ok {
		return
	}
	var req backupTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	req.Bucket = strings.TrimSpace(req.Bucket)
	req.Prefix = strings.Trim(strings.TrimSpace(req.Prefix), "/")
	req.Region = strings.TrimSpace(req.Region)
	req.AccessKey = strings.TrimSpace(req.AccessKey)
	req.SecretKey = strings.TrimSpace(req.SecretKey)
	req.EncryptionKeyID = strings.TrimSpace(req.EncryptionKeyID)
	if req.Endpoint == "" || req.Bucket == "" || req.AccessKey == "" || req.SecretKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint, bucket, accessKey and secretKey are required"})
		return
	}
	if !strings.HasPrefix(strings.ToLower(req.Endpoint), "https://") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "backup S3 endpoint must use HTTPS"})
		return
	}
	secretStore, ok := h.runtime.(runtime.ControlSecretStore)
	if !ok || secretStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "kubernetes control secret store is not available"})
		return
	}
	if err := secretStore.UpsertControlSecret(runtime.SecretSpec{
		Name: backupTargetSecretName,
		Data: map[string]string{
			"endpoint":        req.Endpoint,
			"bucket":          req.Bucket,
			"prefix":          req.Prefix,
			"region":          firstNonEmpty(req.Region, "us-east-1"),
			"accessKey":       req.AccessKey,
			"secretKey":       req.SecretKey,
			"encryptionKeyId": req.EncryptionKeyID,
			"updatedAt":       time.Now().UTC().Format(time.RFC3339),
		},
		Labels: map[string]string{"app.kubernetes.io/name": "noryx-backup", "noryx.io/managed-by": "noryx-backend"},
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save backup target: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "backup.config.update", "backup_config", backupTargetSecretName, "", "success", "", map[string]any{"bucket": req.Bucket, "prefix": req.Prefix})
	status, err := h.backupTargetStatus()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": true})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h Handlers) backupTargetStatus() (backupTargetStatus, error) {
	secretStore, ok := h.runtime.(runtime.ControlSecretStore)
	if !ok || secretStore == nil {
		return backupTargetStatus{}, nil
	}
	data, found, err := secretStore.GetControlSecret(backupTargetSecretName)
	if err != nil || !found {
		return backupTargetStatus{}, err
	}
	updatedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(data["updatedAt"]))
	return backupTargetStatus{
		Configured: true,
		Endpoint:   strings.TrimSpace(data["endpoint"]),
		Bucket:     strings.TrimSpace(data["bucket"]),
		Prefix:     strings.TrimSpace(data["prefix"]),
		Region:     strings.TrimSpace(data["region"]),
		UpdatedAt:  updatedAt,
	}, nil
}

func (h Handlers) requireEnterpriseBackup(w http.ResponseWriter) bool {
	if strings.EqualFold(strings.TrimSpace(h.edition), "enterprise") {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "backup and restore are an Enterprise feature"})
	return false
}
