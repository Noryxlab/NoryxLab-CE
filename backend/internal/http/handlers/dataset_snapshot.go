package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	snapshotdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/snapshot"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// Prendre un instantane, et comparer un dataset a l'un des siens.
//
// Lister un bucket entier coute une requete par millier d'objets et rien
// d'autre : aucun octet n'est lu, aucun n'est copie. C'est ce qui rend
// l'operation acceptable sur de la donnee reglementee, et c'est pour ca
// qu'elle est ouverte a qui peut deja lire le dataset plutot que reservee aux
// administrateurs. Copier, lui, demande `copy-dataset`.

// snapshotListTimeout borne le listage : un bucket enorme ou un stockage
// distant qui ne repond plus doit couter une erreur, pas une requete qui
// pend jusqu'au bout.
const snapshotListTimeout = 10 * time.Minute

type takeSnapshotRequest struct {
	Label string `json:"label"`
}

// TakeDatasetSnapshot records what the dataset holds right now.
func (h Handlers) TakeDatasetSnapshot(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	// Lire le dataset suffit pour en prendre l'empreinte : on n'apprend rien
	// de plus que ce qu'un listage donne deja a qui peut le lire.
	if !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if h.snapshotStore == nil || h.minioClient == nil || strings.TrimSpace(item.Bucket) == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "snapshots need a local dataset and the database store"})
		return
	}
	// Le corps est facultatif : un instantane sans libelle est legitime, et
	// refuser une requete vide ferait echouer le geste le plus simple.
	var req takeSnapshotRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(r.Context(), snapshotListTimeout)
	defer cancel()

	entries := []snapshotdomain.Entry{}
	var total int64
	prefix := strings.Trim(strings.TrimSpace(item.Prefix), "/")
	if prefix != "" {
		prefix += "/"
	}
	for object := range h.minioClient.ListObjects(ctx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list the dataset: " + object.Err.Error()})
			return
		}
		// Un objet dont la cle finit par "/" est un dossier de commodite, pas
		// un fichier : le compter ferait varier l'inventaire selon l'outil qui
		// a depose la donnee.
		if strings.HasSuffix(object.Key, "/") {
			continue
		}
		entries = append(entries, snapshotdomain.Entry{
			Path:         object.Key,
			Size:         object.Size,
			ETag:         strings.Trim(object.ETag, "\""),
			LastModified: object.LastModified.UTC(),
		})
		total += object.Size
	}

	record := snapshotdomain.Snapshot{
		ID:        uuid.NewString(),
		DatasetID: item.ID,
		Label:     strings.TrimSpace(req.Label),
		TakenBy:   identity.UserID(),
		TakenAt:   time.Now().UTC(),
		Objects:   int64(len(entries)),
		TotalSize: total,
	}
	if err := h.snapshotStore.CreateSnapshot(record, entries); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the snapshot"})
		return
	}
	h.emitAudit(r, identity.UserID(), "dataset.snapshot.taken", "dataset", item.ID, "", "success", "",
		map[string]any{"snapshotId": record.ID, "objects": record.Objects, "bytes": record.TotalSize,
			"classification": item.Classification})
	writeJSON(w, http.StatusCreated, record)
}

// ListDatasetSnapshots answers with the readings taken of this dataset.
func (h Handlers) ListDatasetSnapshots(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if h.snapshotStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []snapshotdomain.Snapshot{}})
		return
	}
	items, err := h.snapshotStore.ListSnapshots(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read snapshots"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// CompareDatasetSnapshot says how the dataset differs from one of its
// snapshots.
//
// This is the answer to "has anything been written outside the API": a write
// through a mounted bucket leaves no audit entry, and comparing fingerprints
// is the only thing that sees it.
func (h Handlers) CompareDatasetSnapshot(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if h.snapshotStore == nil || h.minioClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "snapshots need a local dataset and the database store"})
		return
	}
	before, found, err := h.snapshotStore.GetSnapshotEntries(strings.TrimSpace(r.PathValue("snapshotID")), item.ID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "snapshot not found"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), snapshotListTimeout)
	defer cancel()
	now, err := h.listDatasetEntries(ctx, item.Bucket, item.Prefix)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list the dataset: " + err.Error()})
		return
	}
	changes := snapshotdomain.Compare(before, now)
	writeJSON(w, http.StatusOK, map[string]any{
		"changes": changes,
		"total":   len(changes),
		// Dit explicitement, parce qu'une liste vide peut se lire de deux
		// facons et qu'une seule est vraie ici.
		"identical": len(changes) == 0,
	})
}

func (h Handlers) listDatasetEntries(ctx context.Context, bucket, prefix string) ([]snapshotdomain.Entry, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix != "" {
		prefix += "/"
	}
	out := []snapshotdomain.Entry{}
	for object := range h.minioClient.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		if strings.HasSuffix(object.Key, "/") {
			continue
		}
		out = append(out, snapshotdomain.Entry{
			Path:         object.Key,
			Size:         object.Size,
			ETag:         strings.Trim(object.ETag, "\""),
			LastModified: object.LastModified.UTC(),
		})
	}
	return out, nil
}
