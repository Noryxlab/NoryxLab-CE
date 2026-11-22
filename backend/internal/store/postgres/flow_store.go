package postgres

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/flow"
)

func (s *Store) SaveFlow(item flow.Flow) error {
	steps, err := json.Marshal(item.Steps)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO flows (id, project_id, name, description, owner_user_id, steps_json, max_parallel, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, description=EXCLUDED.description,
			steps_json=EXCLUDED.steps_json, max_parallel=EXCLUDED.max_parallel,
			updated_at=EXCLUDED.updated_at`,
		item.ID, item.ProjectID, item.Name, item.Description, item.OwnerUserID,
		steps, item.MaxParallel, item.CreatedAt.UTC(), time.Now().UTC())
	return err
}

func scanFlow(row interface{ Scan(...any) error }) (flow.Flow, error) {
	var item flow.Flow
	var steps []byte
	if err := row.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Description,
		&item.OwnerUserID, &steps, &item.MaxParallel, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return flow.Flow{}, err
	}
	item.Steps = []flow.Step{}
	if len(steps) > 0 {
		_ = json.Unmarshal(steps, &item.Steps)
	}
	return item, nil
}

const flowColumns = `id, project_id, name, description, owner_user_id, steps_json, max_parallel, created_at, updated_at`

func (s *Store) GetFlow(id string) (flow.Flow, bool, error) {
	item, err := scanFlow(s.db.QueryRow(`SELECT `+flowColumns+` FROM flows WHERE id=$1`, strings.TrimSpace(id)))
	if err == sql.ErrNoRows {
		return flow.Flow{}, false, nil
	}
	if err != nil {
		return flow.Flow{}, false, err
	}
	return item, true, nil
}

func (s *Store) ListFlows(projectID string) ([]flow.Flow, error) {
	rows, err := s.db.Query(`SELECT `+flowColumns+` FROM flows WHERE project_id=$1 ORDER BY name`, strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []flow.Flow{}
	for rows.Next() {
		item, err := scanFlow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DeleteFlow(id string) error {
	_, err := s.db.Exec(`DELETE FROM flows WHERE id=$1`, strings.TrimSpace(id))
	return err
}

// CreateRun writes the run and all its steps in one transaction.
//
// A run whose steps were half written would be scheduled on a graph that is
// not the one its author declared - and the scheduler would believe it.
func (s *Store) CreateRun(run flow.Run, steps []flow.StepRun) error {
	transaction, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	if _, err := transaction.Exec(`
		INSERT INTO flow_runs (id, flow_id, project_id, status, started_by, started_at, resume_of)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		run.ID, run.FlowID, run.ProjectID, run.Status, run.StartedBy, run.StartedAt.UTC(), run.ResumeOf); err != nil {
		return err
	}
	statement, err := transaction.Prepare(`
		INSERT INTO flow_step_runs (run_id, step_key, status, job_id, started_at, finished_at, reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, step := range steps {
		if _, err := statement.Exec(run.ID, step.StepKey, step.Status, step.JobID,
			step.StartedAt, step.FinishedAt, step.Reason); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

const flowRunColumns = `id, flow_id, project_id, status, started_by, started_at, finished_at, resume_of`

func scanFlowRun(row interface{ Scan(...any) error }) (flow.Run, error) {
	var item flow.Run
	err := row.Scan(&item.ID, &item.FlowID, &item.ProjectID, &item.Status,
		&item.StartedBy, &item.StartedAt, &item.FinishedAt, &item.ResumeOf)
	return item, err
}

func (s *Store) GetRun(id string) (flow.Run, bool, error) {
	item, err := scanFlowRun(s.db.QueryRow(`SELECT `+flowRunColumns+` FROM flow_runs WHERE id=$1`, strings.TrimSpace(id)))
	if err == sql.ErrNoRows {
		return flow.Run{}, false, nil
	}
	if err != nil {
		return flow.Run{}, false, err
	}
	return item, true, nil
}

func (s *Store) ListRuns(flowID string) ([]flow.Run, error) {
	rows, err := s.db.Query(`SELECT `+flowRunColumns+` FROM flow_runs WHERE flow_id=$1 ORDER BY started_at DESC`, strings.TrimSpace(flowID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []flow.Run{}
	for rows.Next() {
		item, err := scanFlowRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListActiveRuns() ([]flow.Run, error) {
	rows, err := s.db.Query(`SELECT `+flowRunColumns+` FROM flow_runs WHERE status=$1 ORDER BY started_at`, flow.RunRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []flow.Run{}
	for rows.Next() {
		item, err := scanFlowRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRunStatus(runID, status string) error {
	// finished_at est pose par la meme requete que le statut : deux ecritures
	// laisseraient une fenetre ou un run est termine sans date de fin, et
	// c'est exactement ce qu'un ecran lit pour dire « en cours ».
	_, err := s.db.Exec(`
		UPDATE flow_runs SET status=$2,
			finished_at = CASE WHEN $2 IN ($3,$4,$5) THEN now() ELSE finished_at END
		WHERE id=$1`,
		strings.TrimSpace(runID), status, flow.RunSucceeded, flow.RunFailed, flow.RunCancelled)
	return err
}

func (s *Store) ListStepRuns(runID string) ([]flow.StepRun, error) {
	rows, err := s.db.Query(`
		SELECT run_id, step_key, status, job_id, started_at, finished_at, reason
		FROM flow_step_runs WHERE run_id=$1 ORDER BY step_key`, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []flow.StepRun{}
	for rows.Next() {
		var item flow.StepRun
		if err := rows.Scan(&item.RunID, &item.StepKey, &item.Status, &item.JobID,
			&item.StartedAt, &item.FinishedAt, &item.Reason); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStepRun(step flow.StepRun) error {
	_, err := s.db.Exec(`
		UPDATE flow_step_runs SET status=$3, job_id=$4, started_at=$5, finished_at=$6, reason=$7
		WHERE run_id=$1 AND step_key=$2`,
		step.RunID, step.StepKey, step.Status, step.JobID, step.StartedAt, step.FinishedAt, step.Reason)
	return err
}
