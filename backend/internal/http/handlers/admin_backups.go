package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/backup"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

type backupManifest struct {
	SchemaVersion string         `json:"schemaVersion"`
	RunID         string         `json:"runId"`
	GeneratedAt   time.Time      `json:"generatedAt"`
	Instance      map[string]any `json:"instance"`
	Components    map[string]any `json:"components"`
	Inventory     map[string]any `json:"inventory"`
	Notes         []string       `json:"notes"`
}

type backupRunReport struct {
	RunID       string    `json:"runId"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"startedAt"`
	EndedAt     time.Time `json:"endedAt,omitempty"`
	Bucket      string    `json:"bucket"`
	ObjectKey   string    `json:"objectKey"`
	Bytes       int       `json:"bytes"`
	Error       string    `json:"error,omitempty"`
	Warnings    []string  `json:"warnings,omitempty"`
	ManifestSHA string    `json:"manifestSha256,omitempty"`
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

func (h Handlers) ListAdminBackupRuns(w http.ResponseWriter, r *http.Request) {
	if !h.requireEnterpriseBackup(w) {
		return
	}
	if _, ok := h.requireAdminModule(w, r, "backups"); !ok {
		return
	}
	if h.backupRunStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []backup.Run{}})
		return
	}
	items, err := h.backupRunStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list backup runs: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateAdminBackupRun(w http.ResponseWriter, r *http.Request) {
	if !h.requireEnterpriseBackup(w) {
		return
	}
	identity, ok := h.requireAdminModule(w, r, "backups")
	if !ok {
		return
	}
	target, _, err := h.backupTargetData()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read backup target: " + err.Error()})
		return
	}
	if strings.TrimSpace(target["endpoint"]) == "" || strings.TrimSpace(target["bucket"]) == "" || strings.TrimSpace(target["accessKey"]) == "" || strings.TrimSpace(target["secretKey"]) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "backup target is not configured"})
		return
	}
	run := backup.NewRun(identity.UserID(), strings.TrimSpace(target["bucket"]), strings.TrimSpace(target["prefix"]))
	if h.backupRunStore != nil {
		if err := h.backupRunStore.Create(run); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create backup run: " + err.Error()})
			return
		}
	}
	h.emitAudit(r, identity.UserID(), "backup.run.create", "backup_run", run.ID, "", "success", "", map[string]any{"bucket": run.Bucket, "prefix": run.Prefix})
	updated := h.executeManifestBackupRun(r.Context(), run, target)
	if h.backupRunStore != nil {
		_ = h.backupRunStore.Update(updated)
	}
	outcome := "success"
	if updated.Status != "succeeded" {
		outcome = "failure"
	}
	h.emitAudit(r, identity.UserID(), "backup.run."+updated.Status, "backup_run", updated.ID, "", outcome, "", map[string]any{"objectKey": updated.ObjectKey})
	status := http.StatusCreated
	if updated.Status != "succeeded" {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, updated)
}

func (h Handlers) GetAdminBackupRunReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireEnterpriseBackup(w) {
		return
	}
	identity, ok := h.requireAdminModule(w, r, "backups")
	if !ok {
		return
	}
	runID := strings.TrimSpace(r.PathValue("runID"))
	if h.backupRunStore == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "backup run not found"})
		return
	}
	item, found, err := h.backupRunStore.GetByID(runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load backup run: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "backup run not found"})
		return
	}
	h.emitAudit(r, identity.UserID(), "backup.report.download", "backup_run", item.ID, "", "success", "", nil)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-backup-report-`+item.ID+`.json"`)
	if strings.TrimSpace(item.Report) == "" {
		_, _ = w.Write([]byte("{}"))
		return
	}
	_, _ = w.Write([]byte(item.Report))
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
	secretStore, ok := h.runtime.(noryxruntime.ControlSecretStore)
	if !ok || secretStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "kubernetes control secret store is not available"})
		return
	}
	if err := secretStore.UpsertControlSecret(noryxruntime.SecretSpec{
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
	data, found, err := h.backupTargetData()
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

func (h Handlers) backupTargetData() (map[string]string, bool, error) {
	secretStore, ok := h.runtime.(noryxruntime.ControlSecretStore)
	if !ok || secretStore == nil {
		return nil, false, nil
	}
	data, found, err := secretStore.GetControlSecret(backupTargetSecretName)
	return data, found, err
}

func (h Handlers) executeManifestBackupRun(ctx context.Context, run backup.Run, target map[string]string) backup.Run {
	now := time.Now().UTC()
	run.ObjectKey = backupManifestObjectKey(run)
	report := backupRunReport{RunID: run.ID, Status: "running", StartedAt: run.StartedAt, Bucket: run.Bucket, ObjectKey: run.ObjectKey}
	defer func() {
		if run.Report == "" {
			raw, _ := json.MarshalIndent(report, "", "  ")
			run.Report = string(raw)
		}
	}()

	manifest, warnings := h.buildBackupManifest(run)
	report.Warnings = warnings
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		ended := time.Now().UTC()
		run.Status = "failed"
		run.Error = "failed to encode manifest: " + err.Error()
		run.EndedAt = &ended
		report.Status = run.Status
		report.Error = run.Error
		report.EndedAt = ended
		return run
	}
	sum := sha256.Sum256(raw)
	report.ManifestSHA = hex.EncodeToString(sum[:])
	report.Bytes = len(raw)

	client, err := newBackupS3Client(target)
	if err != nil {
		ended := time.Now().UTC()
		run.Status = "failed"
		run.Error = err.Error()
		run.EndedAt = &ended
		report.Status = run.Status
		report.Error = run.Error
		report.EndedAt = ended
		return run
	}
	uploadCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if _, err := client.PutObject(uploadCtx, run.Bucket, run.ObjectKey, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{ContentType: "application/json"}); err != nil {
		ended := time.Now().UTC()
		run.Status = "failed"
		run.Error = "failed to upload manifest: " + err.Error()
		run.EndedAt = &ended
		report.Status = run.Status
		report.Error = run.Error
		report.EndedAt = ended
		return run
	}
	run.Status = "succeeded"
	run.EndedAt = &now
	report.Status = run.Status
	report.EndedAt = now
	rawReport, _ := json.MarshalIndent(report, "", "  ")
	run.Report = string(rawReport)
	return run
}

func (h Handlers) buildBackupManifest(run backup.Run) (backupManifest, []string) {
	warnings := []string{"first implementation: manifest-only backup; postgres dump, secrets export and local MinIO object copy are not included yet"}
	projects, projectErr := h.projectStore.List()
	datasets, datasetErr := h.datasetStore.ListAll()
	datasources, datasourceErr := h.datasourceStore.ListAll()
	ontologies, ontologyErr := h.ontologyStore.ListAll()
	apps, appErr := h.appStore.List()
	jobs, jobErr := h.jobStore.List()
	workspaces, workspaceErr := h.workspaceStore.List()
	builds, buildErr := h.buildStore.List()
	if projectErr != nil {
		warnings = append(warnings, "projects count unavailable: "+projectErr.Error())
	}
	if datasetErr != nil {
		warnings = append(warnings, "datasets count unavailable: "+datasetErr.Error())
	}
	if datasourceErr != nil {
		warnings = append(warnings, "datasources count unavailable: "+datasourceErr.Error())
	}
	if ontologyErr != nil {
		warnings = append(warnings, "ontologies count unavailable: "+ontologyErr.Error())
	}
	if appErr != nil {
		warnings = append(warnings, "apps count unavailable: "+appErr.Error())
	}
	if jobErr != nil {
		warnings = append(warnings, "jobs count unavailable: "+jobErr.Error())
	}
	if workspaceErr != nil {
		warnings = append(warnings, "workspaces count unavailable: "+workspaceErr.Error())
	}
	if buildErr != nil {
		warnings = append(warnings, "builds count unavailable: "+buildErr.Error())
	}
	deployments := []noryxruntime.DeploymentStatus{}
	services := []noryxruntime.ServiceStatus{}
	if inspector, ok := h.runtime.(noryxruntime.Inspector); ok && inspector != nil {
		if items, err := inspector.ListDeployments(); err == nil {
			deployments = items
		} else {
			warnings = append(warnings, "deployments inventory unavailable: "+err.Error())
		}
		if items, err := inspector.ListServices(); err == nil {
			services = items
		} else {
			warnings = append(warnings, "services inventory unavailable: "+err.Error())
		}
	}
	userCount := 0
	if h.keycloak != nil {
		if users, err := h.keycloak.ListUsers(); err == nil {
			userCount = len(users)
		} else {
			warnings = append(warnings, "users count unavailable: "+err.Error())
		}
	}
	return backupManifest{
		SchemaVersion: "noryx-backup-manifest-v1",
		RunID:         run.ID,
		GeneratedAt:   time.Now().UTC(),
		Instance: map[string]any{
			"edition":        h.edition,
			"backendVersion": h.backendVersion,
			"defaultTheme":   h.defaultTheme,
		},
		Components: map[string]any{
			"deployments": deployments,
			"services":    services,
		},
		Inventory: map[string]any{
			"users":       userCount,
			"projects":    len(projects),
			"datasets":    len(datasets),
			"datasources": len(datasources),
			"ontologies":  len(ontologies),
			"apps":        len(apps),
			"jobs":        len(jobs),
			"workspaces":  len(workspaces),
			"builds":      len(builds),
		},
		Notes: warnings,
	}, warnings
}

func backupManifestObjectKey(run backup.Run) string {
	stamp := run.StartedAt.UTC().Format("20060102T150405Z")
	prefix := strings.Trim(strings.TrimSpace(run.Prefix), "/")
	key := strings.Trim(strings.Join([]string{prefix, "manual", stamp, run.ID, "manifest.json"}, "/"), "/")
	return key
}

func newBackupS3Client(target map[string]string) (*minio.Client, error) {
	endpoint := strings.TrimSpace(target["endpoint"])
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("backup S3 endpoint must be a valid HTTPS URL")
	}
	return minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(target["accessKey"]), strings.TrimSpace(target["secretKey"]), ""),
		Secure: true,
		Region: strings.TrimSpace(target["region"]),
	})
}

func (h Handlers) requireEnterpriseBackup(w http.ResponseWriter) bool {
	if strings.EqualFold(strings.TrimSpace(h.edition), "enterprise") {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "backup and restore are an Enterprise feature"})
	return false
}
