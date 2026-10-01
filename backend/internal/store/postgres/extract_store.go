package postgres

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
)

// ExtractStore persists an extract and the file list it froze.
//
// The filter is kept for the record - it is how a person recognises what they
// asked for - but it is the frozen list that answers "which files", because
// re-running the filter next month would quietly return a different study.
type ExtractStore struct{ *Store }

func (s *ExtractStore) ListByProject(projectID string) ([]extract.Extract, error) {
	rows, err := s.db.Query(`SELECT id, ontology_id, project_id, owner_user_id, owner_type, owner_id, name, description, subjects_json, modalities_json, visits_json, layout_json, object_count, total_bytes, created_at, updated_at FROM extracts WHERE project_id=$1 ORDER BY created_at DESC`, strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []extract.Extract{}
	for rows.Next() {
		item, err := scanExtract(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *ExtractStore) ListByOntology(ontologyID string) ([]extract.Extract, error) {
	rows, err := s.db.Query(`SELECT id, ontology_id, project_id, owner_user_id, owner_type, owner_id, name, description, subjects_json, modalities_json, visits_json, layout_json, object_count, total_bytes, created_at, updated_at FROM extracts WHERE ontology_id=$1 ORDER BY created_at DESC`, strings.TrimSpace(ontologyID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []extract.Extract{}
	for rows.Next() {
		item, err := scanExtract(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *ExtractStore) GetByID(id string) (extract.Extract, bool, error) {
	row := s.db.QueryRow(`SELECT id, ontology_id, project_id, owner_user_id, owner_type, owner_id, name, description, subjects_json, modalities_json, visits_json, layout_json, object_count, total_bytes, created_at, updated_at FROM extracts WHERE id=$1`, strings.TrimSpace(id))
	item, err := scanExtract(row)
	if err == sql.ErrNoRows {
		return extract.Extract{}, false, nil
	}
	if err != nil {
		return extract.Extract{}, false, err
	}
	return item, true, nil
}

// Create writes the extract and its frozen members in one transaction: a
// half-written extract would report an n it cannot list.
func (s *ExtractStore) Create(item extract.Extract, members []extract.Member) error {
	subjects, _ := json.Marshal(item.Subjects)
	modalities, _ := json.Marshal(item.Modalities)
	visits, _ := json.Marshal(item.Visits)
	layout, _ := json.Marshal(item.Layout)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO extracts (id, ontology_id, project_id, owner_user_id, owner_type, owner_id, name, description, subjects_json, modalities_json, visits_json, layout_json, object_count, total_bytes, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		item.ID, item.OntologyID, item.ProjectID, item.OwnerUserID, item.OwnerType, item.OwnerID, item.Name, item.Description, subjects, modalities, visits, layout, item.ObjectCount, item.TotalBytes, item.CreatedAt, item.UpdatedAt); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO extract_members (extract_id, path, subject_id, visit, modality, size_bytes) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (extract_id, path) DO NOTHING`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, member := range members {
		if _, err := statement.Exec(item.ID, member.Path, member.SubjectID, member.Visit, member.Modality, member.SizeBytes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *ExtractStore) ListMembers(extractID string, limit int) ([]extract.Member, error) {
	query := `SELECT extract_id, path, subject_id, visit, modality, size_bytes FROM extract_members WHERE extract_id=$1 ORDER BY subject_id, visit, modality, path`
	args := []any{strings.TrimSpace(extractID)}
	if limit > 0 {
		args = append(args, limit)
		query += ` LIMIT $2`
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []extract.Member{}
	for rows.Next() {
		var member extract.Member
		if err := rows.Scan(&member.ExtractID, &member.Path, &member.SubjectID, &member.Visit, &member.Modality, &member.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, member)
	}
	return out, rows.Err()
}

func (s *ExtractStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM extracts WHERE id=$1`, strings.TrimSpace(id))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanExtract(row rowScanner) (extract.Extract, error) {
	var item extract.Extract
	var subjects, modalities, visits, layout []byte
	var created, updated time.Time
	if err := row.Scan(&item.ID, &item.OntologyID, &item.ProjectID, &item.OwnerUserID, &item.OwnerType, &item.OwnerID, &item.Name, &item.Description, &subjects, &modalities, &visits, &layout, &item.ObjectCount, &item.TotalBytes, &created, &updated); err != nil {
		return extract.Extract{}, err
	}
	item.CreatedAt = created
	item.UpdatedAt = updated
	// An unreadable filter must not hide the extract: the frozen member list is
	// what the extract *is*, and the filter is a record of how it was asked for.
	_ = json.Unmarshal(subjects, &item.Subjects)
	_ = json.Unmarshal(modalities, &item.Modalities)
	_ = json.Unmarshal(visits, &item.Visits)
	_ = json.Unmarshal(layout, &item.Layout)
	if item.Subjects == nil {
		item.Subjects = []string{}
	}
	if item.Modalities == nil {
		item.Modalities = []string{}
	}
	if item.Visits == nil {
		item.Visits = []string{}
	}
	return item, nil
}

// UpdateMetadata corrects the label. The frozen file list, the author and the
// dates are what the extract is, and renaming does not touch any of them.
func (s *ExtractStore) UpdateMetadata(id, name, description string) error {
	result, err := s.db.Exec(
		`UPDATE extracts SET name=$2, description=$3, updated_at=NOW() WHERE id=$1`,
		strings.TrimSpace(id), strings.TrimSpace(name), strings.TrimSpace(description))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SetOwner hands the extract to somebody else, and touches nothing else.
func (s *ExtractStore) SetOwner(id, ownerType, ownerID string) error {
	result, err := s.db.Exec(
		`UPDATE extracts SET owner_type=$2, owner_id=$3, updated_at=NOW() WHERE id=$1`,
		strings.TrimSpace(id), strings.TrimSpace(ownerType), strings.TrimSpace(ownerID))
	if err != nil {
		return err
	}
	// Dire qu on n a rien trouve plutot que de rendre un succes silencieux :
	// un transfert vers un identifiant qui n existe pas doit se voir.
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
