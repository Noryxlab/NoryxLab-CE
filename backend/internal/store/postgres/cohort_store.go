package postgres

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/cohort"
)

// CohortStore persists a cohort and the file list it froze.
//
// The filter is kept for the record - it is how a person recognises what they
// asked for - but it is the frozen list that answers "which files", because
// re-running the filter next month would quietly return a different study.
type CohortStore struct{ *Store }

func (s *CohortStore) ListByProject(projectID string) ([]cohort.Cohort, error) {
	rows, err := s.db.Query(`SELECT id, ontology_id, project_id, owner_user_id, name, description, subjects_json, modalities_json, visits_json, object_count, total_bytes, created_at, updated_at FROM cohorts WHERE project_id=$1 ORDER BY created_at DESC`, strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []cohort.Cohort{}
	for rows.Next() {
		item, err := scanCohort(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CohortStore) ListByOntology(ontologyID string) ([]cohort.Cohort, error) {
	rows, err := s.db.Query(`SELECT id, ontology_id, project_id, owner_user_id, name, description, subjects_json, modalities_json, visits_json, object_count, total_bytes, created_at, updated_at FROM cohorts WHERE ontology_id=$1 ORDER BY created_at DESC`, strings.TrimSpace(ontologyID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []cohort.Cohort{}
	for rows.Next() {
		item, err := scanCohort(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CohortStore) GetByID(id string) (cohort.Cohort, bool, error) {
	row := s.db.QueryRow(`SELECT id, ontology_id, project_id, owner_user_id, name, description, subjects_json, modalities_json, visits_json, object_count, total_bytes, created_at, updated_at FROM cohorts WHERE id=$1`, strings.TrimSpace(id))
	item, err := scanCohort(row)
	if err == sql.ErrNoRows {
		return cohort.Cohort{}, false, nil
	}
	if err != nil {
		return cohort.Cohort{}, false, err
	}
	return item, true, nil
}

// Create writes the cohort and its frozen members in one transaction: a
// half-written cohort would report an n it cannot list.
func (s *CohortStore) Create(item cohort.Cohort, members []cohort.Member) error {
	subjects, _ := json.Marshal(item.Subjects)
	modalities, _ := json.Marshal(item.Modalities)
	visits, _ := json.Marshal(item.Visits)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO cohorts (id, ontology_id, project_id, owner_user_id, name, description, subjects_json, modalities_json, visits_json, object_count, total_bytes, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		item.ID, item.OntologyID, item.ProjectID, item.OwnerUserID, item.Name, item.Description, subjects, modalities, visits, item.ObjectCount, item.TotalBytes, item.CreatedAt, item.UpdatedAt); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO cohort_members (cohort_id, path, subject_id, visit, modality, size_bytes) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (cohort_id, path) DO NOTHING`)
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

func (s *CohortStore) ListMembers(cohortID string, limit int) ([]cohort.Member, error) {
	query := `SELECT cohort_id, path, subject_id, visit, modality, size_bytes FROM cohort_members WHERE cohort_id=$1 ORDER BY subject_id, visit, modality, path`
	args := []any{strings.TrimSpace(cohortID)}
	if limit > 0 {
		args = append(args, limit)
		query += ` LIMIT $2`
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []cohort.Member{}
	for rows.Next() {
		var member cohort.Member
		if err := rows.Scan(&member.CohortID, &member.Path, &member.SubjectID, &member.Visit, &member.Modality, &member.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, member)
	}
	return out, rows.Err()
}

func (s *CohortStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM cohorts WHERE id=$1`, strings.TrimSpace(id))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCohort(row rowScanner) (cohort.Cohort, error) {
	var item cohort.Cohort
	var subjects, modalities, visits []byte
	var created, updated time.Time
	if err := row.Scan(&item.ID, &item.OntologyID, &item.ProjectID, &item.OwnerUserID, &item.Name, &item.Description, &subjects, &modalities, &visits, &item.ObjectCount, &item.TotalBytes, &created, &updated); err != nil {
		return cohort.Cohort{}, err
	}
	item.CreatedAt = created
	item.UpdatedAt = updated
	// An unreadable filter must not hide the cohort: the frozen member list is
	// what the cohort *is*, and the filter is a record of how it was asked for.
	_ = json.Unmarshal(subjects, &item.Subjects)
	_ = json.Unmarshal(modalities, &item.Modalities)
	_ = json.Unmarshal(visits, &item.Visits)
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
