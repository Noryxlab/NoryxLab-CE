package postgres

import (
	"database/sql"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/snapshot"
)

// Le manifeste est ecrit en une transaction, parce qu'un instantane a moitie
// ecrit mentirait deux fois : il affirmerait un compte d'objets qu'il ne porte
// pas, et une comparaison ulterieure signalerait comme « supprimes » des
// fichiers qui n'ont jamais ete lus.
func (s *Store) CreateSnapshot(item snapshot.Snapshot, entries []snapshot.Entry) error {
	transaction, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()

	if _, err := transaction.Exec(`
		INSERT INTO dataset_snapshots (id, dataset_id, label, taken_by, taken_at, objects, total_size, copy_dataset_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		item.ID, item.DatasetID, item.Label, item.TakenBy, item.TakenAt.UTC(),
		item.Objects, item.TotalSize, item.CopyDatasetID); err != nil {
		return err
	}
	statement, err := transaction.Prepare(`
		INSERT INTO dataset_snapshot_objects (snapshot_id, path, size, etag, last_modified)
		VALUES ($1,$2,$3,$4,$5)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, entry := range entries {
		if _, err := statement.Exec(item.ID, entry.Path, entry.Size, entry.ETag, entry.LastModified.UTC()); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (s *Store) ListSnapshots(datasetID string) ([]snapshot.Snapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, dataset_id, label, taken_by, taken_at, objects, total_size, copy_dataset_id
		FROM dataset_snapshots WHERE dataset_id=$1 ORDER BY taken_at DESC`, strings.TrimSpace(datasetID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []snapshot.Snapshot{}
	for rows.Next() {
		var item snapshot.Snapshot
		if err := rows.Scan(&item.ID, &item.DatasetID, &item.Label, &item.TakenBy,
			&item.TakenAt, &item.Objects, &item.TotalSize, &item.CopyDatasetID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// DeleteSnapshot removes a reading, manifest included.
//
// The manifest goes with it through the foreign key's cascade rather than a
// second statement here: two deletes would leave orphan object rows behind
// whenever the second one failed, and nothing would ever look for them.
//
// Answers whether a row was actually removed, so a caller can tell "deleted"
// from "there was nothing there" - and so a snapshot identifier belonging to
// another dataset answers the second rather than the first.
func (s *Store) DeleteSnapshot(snapshotID, datasetID string) (bool, error) {
	result, err := s.db.Exec(`DELETE FROM dataset_snapshots WHERE id=$1 AND dataset_id=$2`,
		strings.TrimSpace(snapshotID), strings.TrimSpace(datasetID))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) GetSnapshotEntries(snapshotID, datasetID string) ([]snapshot.Entry, bool, error) {
	var exists string
	err := s.db.QueryRow(`SELECT id FROM dataset_snapshots WHERE id=$1 AND dataset_id=$2`,
		strings.TrimSpace(snapshotID), strings.TrimSpace(datasetID)).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := s.db.Query(`SELECT path, size, etag, last_modified FROM dataset_snapshot_objects WHERE snapshot_id=$1`,
		strings.TrimSpace(snapshotID))
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []snapshot.Entry{}
	for rows.Next() {
		var entry snapshot.Entry
		if err := rows.Scan(&entry.Path, &entry.Size, &entry.ETag, &entry.LastModified); err != nil {
			return nil, false, err
		}
		out = append(out, entry)
	}
	return out, true, rows.Err()
}

func (s *Store) LatestSnapshot(datasetID string) (snapshot.Snapshot, bool, error) {
	var item snapshot.Snapshot
	err := s.db.QueryRow(`
		SELECT id, dataset_id, label, taken_by, taken_at, objects, total_size, copy_dataset_id
		FROM dataset_snapshots WHERE dataset_id=$1 ORDER BY taken_at DESC LIMIT 1`,
		strings.TrimSpace(datasetID)).Scan(&item.ID, &item.DatasetID, &item.Label, &item.TakenBy,
		&item.TakenAt, &item.Objects, &item.TotalSize, &item.CopyDatasetID)
	if err == sql.ErrNoRows {
		return snapshot.Snapshot{}, false, nil
	}
	if err != nil {
		return snapshot.Snapshot{}, false, err
	}
	return item, true, nil
}

var _ = time.Now
