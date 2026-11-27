package postgres

import (
	"database/sql"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/attestation"
)

type AttestationStore struct{ *Store }

func (s *AttestationStore) Create(item attestation.Attestation) error {
	_, err := s.db.Exec(`
		INSERT INTO training_attestations
			(id, user_id, subject, file_name, content_type, size, storage_key, valid_until, uploaded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		item.ID, item.UserID, item.Subject, item.FileName, item.ContentType,
		item.Size, item.StorageKey, optionalTime(item.ValidUntil), item.UploadedAt.UTC())
	return err
}

func (s *AttestationStore) ListByUser(userID string) ([]attestation.Attestation, error) {
	return s.query(`
		SELECT id, user_id, subject, file_name, content_type, size, storage_key, valid_until, uploaded_at
		  FROM training_attestations WHERE user_id=$1 ORDER BY uploaded_at DESC`, userID)
}

func (s *AttestationStore) ListAll() ([]attestation.Attestation, error) {
	return s.query(`
		SELECT id, user_id, subject, file_name, content_type, size, storage_key, valid_until, uploaded_at
		  FROM training_attestations ORDER BY uploaded_at DESC`)
}

func (s *AttestationStore) GetByID(id string) (attestation.Attestation, bool, error) {
	items, err := s.query(`
		SELECT id, user_id, subject, file_name, content_type, size, storage_key, valid_until, uploaded_at
		  FROM training_attestations WHERE id=$1`, id)
	if err != nil || len(items) == 0 {
		return attestation.Attestation{}, false, err
	}
	return items[0], true, nil
}

func (s *AttestationStore) Delete(id string) (bool, error) {
	result, err := s.db.Exec(`DELETE FROM training_attestations WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *AttestationStore) query(statement string, args ...any) ([]attestation.Attestation, error) {
	rows, err := s.db.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []attestation.Attestation{}
	for rows.Next() {
		var item attestation.Attestation
		var validUntil sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.Subject, &item.FileName,
			&item.ContentType, &item.Size, &item.StorageKey, &validUntil, &item.UploadedAt); err != nil {
			return nil, err
		}
		if validUntil.Valid {
			moment := validUntil.Time.UTC()
			item.ValidUntil = &moment
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// optionalTime, and not the nullableTime already in this package: that one
// takes a time and nulls the zero value, which is the right shape for a
// column that is "unset or a moment". Here the absence is a nil pointer and
// carries its own meaning - the person did not say when the training runs
// out - so the two must not be confused.
func optionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
