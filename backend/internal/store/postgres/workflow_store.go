package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workflow"
)

type WorkflowStore struct{ Store *Store }

// Steps and step runs are stored as JSON documents on their row.
//
// A step is read and written as part of its workflow, never alone, and a run
// is persisted whole after every step; a normalised table per step would buy
// nothing but a join on the path the scheduler walks every five minutes.

func (s *WorkflowStore) Create(item workflow.Definition) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	steps, err := json.Marshal(item.Steps)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		INSERT INTO workflows (id, owner_user_id, project_id, name, schedule, steps_json, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		item.ID, item.OwnerUserID, item.ProjectID, item.Name, item.Schedule, string(steps),
		item.Enabled, item.CreatedAt.UTC(), item.UpdatedAt.UTC())
	return err
}

func (s *WorkflowStore) Update(item workflow.Definition) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	steps, err := json.Marshal(item.Steps)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		UPDATE workflows SET project_id=$2, name=$3, schedule=$4, steps_json=$5, enabled=$6,
		       updated_at=$7, last_run_at=$8
		WHERE id=$1`,
		item.ID, item.ProjectID, item.Name, item.Schedule, string(steps), item.Enabled,
		time.Now().UTC(), item.LastRunAt)
	return err
}

func (s *WorkflowStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The runs go with it, for the reason an agent's do: a history whose
	// workflow no longer exists can be read by nobody.
	if _, err := s.Store.db.ExecContext(ctx, `DELETE FROM workflow_runs WHERE workflow_id=$1`, id); err != nil {
		return err
	}
	_, err := s.Store.db.ExecContext(ctx, `DELETE FROM workflows WHERE id=$1`, id)
	return err
}

func (s *WorkflowStore) GetByID(id string) (workflow.Definition, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := s.Store.db.QueryRowContext(ctx, workflowSelect+` WHERE id=$1`, id)
	item, err := scanWorkflow(row)
	if err == sql.ErrNoRows {
		return workflow.Definition{}, false, nil
	}
	return item, err == nil, err
}

func (s *WorkflowStore) ListByOwner(ownerUserID string) ([]workflow.Definition, error) {
	return s.list(workflowSelect+` WHERE owner_user_id=$1 ORDER BY created_at`, ownerUserID)
}

func (s *WorkflowStore) ListAll() ([]workflow.Definition, error) {
	return s.list(workflowSelect + ` ORDER BY created_at`)
}

func (s *WorkflowStore) list(query string, args ...any) ([]workflow.Definition, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []workflow.Definition{}
	for rows.Next() {
		item, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const workflowSelect = `SELECT id, owner_user_id, project_id, name, schedule, steps_json, enabled,
	created_at, updated_at, last_run_at FROM workflows`

func scanWorkflow(row rowScanner) (workflow.Definition, error) {
	var item workflow.Definition
	var steps string
	var lastRun sql.NullTime
	if err := row.Scan(&item.ID, &item.OwnerUserID, &item.ProjectID, &item.Name, &item.Schedule,
		&steps, &item.Enabled, &item.CreatedAt, &item.UpdatedAt, &lastRun); err != nil {
		return workflow.Definition{}, err
	}
	if lastRun.Valid {
		at := lastRun.Time.UTC()
		item.LastRunAt = &at
	}
	item.Steps = []workflow.Step{}
	_ = json.Unmarshal([]byte(steps), &item.Steps)
	// Re-normalised on the way out, like an agent's actions: a row written by
	// a version that knew a grant this one has withdrawn must not grant it.
	item.Steps = workflow.NormaliseSteps(item.Steps)
	return item, nil
}

func (s *WorkflowStore) CreateRun(run workflow.Run) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		INSERT INTO workflow_runs (id, workflow_id, status, steps_json, error, started_at, updated_at, finished_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		run.ID, run.WorkflowID, run.Status, string(steps), run.Error,
		run.StartedAt.UTC(), run.UpdatedAt.UTC(), run.FinishedAt)
	return err
}

func (s *WorkflowStore) UpdateRun(run workflow.Run) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	steps, err := json.Marshal(run.Steps)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		UPDATE workflow_runs SET status=$2, steps_json=$3, error=$4, updated_at=$5, finished_at=$6 WHERE id=$1`,
		run.ID, run.Status, string(steps), run.Error, run.UpdatedAt.UTC(), run.FinishedAt)
	return err
}

const runSelect = `SELECT id, workflow_id, status, steps_json, error, started_at, updated_at, finished_at FROM workflow_runs`

func (s *WorkflowStore) GetRun(id string) (workflow.Run, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run, err := scanRun(s.Store.db.QueryRowContext(ctx, runSelect+` WHERE id=$1`, id))
	if err == sql.ErrNoRows {
		return workflow.Run{}, false, nil
	}
	return run, err == nil, err
}

func (s *WorkflowStore) ListRuns(workflowID string, limit int) ([]workflow.Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.listRuns(runSelect+` WHERE workflow_id=$1 ORDER BY started_at DESC LIMIT $2`, workflowID, limit)
}

func (s *WorkflowStore) ListUnfinishedRuns() ([]workflow.Run, error) {
	return s.listRuns(runSelect+` WHERE status IN ($1,$2) ORDER BY started_at`,
		workflow.StatusPending, workflow.StatusRunning)
}

func (s *WorkflowStore) listRuns(query string, args ...any) ([]workflow.Run, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []workflow.Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func scanRun(row rowScanner) (workflow.Run, error) {
	var run workflow.Run
	var steps string
	var finished sql.NullTime
	if err := row.Scan(&run.ID, &run.WorkflowID, &run.Status, &steps, &run.Error,
		&run.StartedAt, &run.UpdatedAt, &finished); err != nil {
		return workflow.Run{}, err
	}
	if finished.Valid {
		at := finished.Time.UTC()
		run.FinishedAt = &at
	}
	run.Steps = []workflow.StepRun{}
	_ = json.Unmarshal([]byte(steps), &run.Steps)
	for index := range run.Steps {
		if run.Steps[index].Actions == nil {
			run.Steps[index].Actions = []string{}
		}
	}
	return run, nil
}
