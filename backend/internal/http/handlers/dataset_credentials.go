package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
)

// Replacing the credentials a dataset uses to reach its object store.
//
// They could be set when the dataset was created and never afterwards, which
// held until the day the object store's password was rotated. Every dataset
// carries its own encrypted copy, so all of them stopped being readable at
// once, and the only way back was to recreate them - losing the access rules
// already granted on them - or to rewrite the database by hand.
//
// It happened on 2026-09-17, to five datasets, four of them regulated. The
// rotation had updated the consumers somebody thought to list; datasets were
// a fifth nobody had.
//
// The new credentials are proved before they are stored. Saving one that does
// not work only moves the failure somewhere less visible: the screen says
// saved, and the dataset stays unreadable until the next person to open it
// wonders why.

const credentialProbeTimeout = 8 * time.Second

type datasetCredentialRequest struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

func (h Handlers) UpdateDatasetCredentials(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	datasetID := strings.TrimSpace(r.PathValue("datasetID"))
	item, found, err := h.datasetStore.GetByID(datasetID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	// The same gate as changing who owns it: this is a key to the data, not a
	// property of it.
	if !h.canManageDatasetAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "dataset owner or global admin required"})
		return
	}
	if item.Provider == "minio" {
		// The internal store's credentials belong to the platform, not to one
		// dataset, and rewriting them here would desynchronise every other one.
		writeJSON(w, http.StatusConflict, map[string]string{"error": "datasets on the internal object store do not carry their own credentials"})
		return
	}

	var req datasetCredentialRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "accessKey and secretKey are required"})
		return
	}
	req.AccessKey = strings.TrimSpace(req.AccessKey)
	req.SecretKey = strings.TrimSpace(req.SecretKey)
	if req.AccessKey == "" || req.SecretKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "accessKey and secretKey are required"})
		return
	}

	if err := h.probeDatasetCredentials(item, req); err != nil {
		h.emitAdvancedAudit(r, identity.UserID(), "dataset.credentials.update", "dataset", item.ID, "", "failure", "unusable", nil)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "these credentials cannot reach the bucket, so they were not saved: " + err.Error(),
		})
		return
	}

	payload, err := json.Marshal(datasetS3Credential{AccessKey: req.AccessKey, SecretKey: req.SecretKey})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to prepare S3 credentials"})
		return
	}
	encrypted, err := security.EncryptString(h.secretsMasterKey, string(payload))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt S3 credentials"})
		return
	}
	credentialUserID := item.CredentialUserID
	if credentialUserID == "" {
		credentialUserID = item.OwnerUserID
	}
	name := item.CredentialName
	if name == "" {
		name = "dataset-s3-" + item.ID
	}
	if err := h.secretStore.Upsert(secret.New(credentialUserID, name, "dataset-s3", encrypted)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store encrypted S3 credentials"})
		return
	}

	// The access key is recorded, never the secret: an audit trail that holds
	// the credential is a second place to leak it from.
	h.emitAdvancedAudit(r, identity.UserID(), "dataset.credentials.update", "dataset", item.ID, "", "success", "",
		map[string]any{"accessKey": req.AccessKey, "bucket": item.Bucket})
	writeJSON(w, http.StatusOK, map[string]any{"updated": true})
}

// probeDatasetCredentials asks whether the bucket answers, without reading it.
//
// BucketExists rather than listing one object: it proves the endpoint, the
// credentials and the access to that bucket, and it enumerates nothing. For a
// regulated dataset that distinction is the whole point - the platform should
// be able to prove it still has the key without walking through the door.
func (h Handlers) probeDatasetCredentials(item dataset.Dataset, req datasetCredentialRequest) error {
	client, err := newDedicatedDatasetS3Client(item.Endpoint, item.Region, req.AccessKey, req.SecretKey)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), credentialProbeTimeout)
	defer cancel()
	exists, err := client.BucketExists(ctx, item.Bucket)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("the bucket does not exist, or these credentials cannot see it")
	}
	return nil
}
