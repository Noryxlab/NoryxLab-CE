package handlers

import (
	"context"
	"encoding/json"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/objectstore"
	"net/http"
	"strings"
	"time"

	snapshotdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/snapshot"
	"github.com/google/uuid"
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
	if h.snapshotStore == nil || strings.TrimSpace(item.Bucket) == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "snapshots need a dataset with a bucket and the database store"})
		return
	}
	store, err := h.datasetObjectStore(item)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	// Le corps est facultatif : un instantane sans libelle est legitime, et
	// refuser une requete vide ferait echouer le geste le plus simple.
	var req takeSnapshotRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(r.Context(), snapshotListTimeout)
	defer cancel()

	// La meme lecture que le scan de nuit et que le croisement, par la meme
	// fonction. Elle etait recopiee ici, et les deux copies avaient deja
	// divergé sur un detail qui compte : celle-ci comptait les dossiers de
	// commodite, l'autre non, donc un instantane pouvait ne pas s'accorder
	// avec sa propre comparaison du lendemain.
	entries, err := listDatasetEntries(ctx, store, item.Bucket, item.Prefix)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list the dataset: " + err.Error()})
		return
	}
	var total int64
	for _, entry := range entries {
		total += entry.Size
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

// DeleteDatasetSnapshot removes one reading.
//
// Writing, not reading, and that is the decision. Taking a snapshot is open to
// whoever can read the dataset - it learns nothing a listing does not already
// give them. Removing one is the opposite act: the last snapshot of a dataset
// is what enrols it in the nightly drift check, so deleting it stops the
// surveillance. That belongs to whoever may change the dataset.
func (h Handlers) DeleteDatasetSnapshot(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	if !h.canWriteDataset(item, identity) {
		// Dit en entier : un refus qui n'annonce que « interdit » laisse
		// chercher le droit manquant, et celui-la n'est pas celui qu'on
		// attend apres avoir pu prendre l'instantane soi-meme.
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "removing a snapshot needs write access to the dataset: " +
				"the last snapshot is what enrols it in the nightly drift check, " +
				"so deleting it stops that check"})
		return
	}
	if h.snapshotStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "snapshots need the database store"})
		return
	}
	removed, err := h.snapshotStore.DeleteSnapshot(strings.TrimSpace(r.PathValue("snapshotID")), item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete the snapshot"})
		return
	}
	if !removed {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "snapshot not found"})
		return
	}
	h.emitAudit(r, identity.UserID(), "dataset.snapshot.deleted", "dataset", item.ID, "", "success", "",
		map[string]any{"snapshotId": strings.TrimSpace(r.PathValue("snapshotID")),
			"classification": item.Classification})
	w.WriteHeader(http.StatusNoContent)
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
	if h.snapshotStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "snapshots need the database store"})
		return
	}
	store, err := h.datasetObjectStore(item)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	before, found, err := h.snapshotStore.GetSnapshotEntries(strings.TrimSpace(r.PathValue("snapshotID")), item.ID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "snapshot not found"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), snapshotListTimeout)
	defer cancel()
	now, err := listDatasetEntries(ctx, store, item.Bucket, item.Prefix)
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

// listDatasetEntries reads a dataset through its own storage client.
//
// Its own, not the platform's: a dataset on an external S3 endpoint has its
// own address and its own credential, and listing it through the internal
// MinIO client asked that client for a bucket of the same name - which either
// does not exist or, worse, is somebody else's. The regulated datasets at the
// customer site are exactly the ones that are external, so this was the case
// that mattered and the one that would have failed.
func listDatasetEntries(ctx context.Context, store objectstore.Store, bucket, prefix string) ([]snapshotdomain.Entry, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix != "" {
		prefix += "/"
	}
	out := []snapshotdomain.Entry{}
	err := store.List(ctx, bucket, objectstore.ListOptions{Prefix: prefix, Recursive: true}, func(object objectstore.Object) error {
		// A recursive listing should hold no folder markers, but a bucket
		// written by another tool can carry zero-byte keys ending in a
		// slash, and counting those as objects makes a manifest disagree
		// with itself between two listings.
		if strings.HasSuffix(object.Path, "/") {
			return nil
		}
		out = append(out, snapshotdomain.Entry{
			Path:         object.Path,
			Size:         object.Size,
			ETag:         object.ETag,
			LastModified: object.LastModified,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
