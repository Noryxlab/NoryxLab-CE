package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

type AgentStore struct{ Store *Store }

// Actions are stored as JSON rather than as a comma-joined string.
//
// The same shortcut in the token store meant a scope containing a comma would
// have split into two; here it would mean an action name splitting into two
// names, one of which might match something real. A list is stored as a list.

func (s *AgentStore) Create(item agent.Agent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actions, err := json.Marshal(item.Actions)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		INSERT INTO agents (id, owner_user_id, project_id, name, mission, schedule, actions_json, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		item.ID, item.OwnerUserID, item.ProjectID, item.Name, item.Mission, item.Schedule,
		string(actions), item.Enabled, item.CreatedAt.UTC(), item.UpdatedAt.UTC())
	return err
}

func (s *AgentStore) Update(item agent.Agent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actions, err := json.Marshal(item.Actions)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		UPDATE agents SET project_id=$2, name=$3, mission=$4, schedule=$5, actions_json=$6,
		       enabled=$7, updated_at=$8, last_run_at=$9, last_report=$10, last_quiet=$11
		WHERE id=$1`,
		item.ID, item.ProjectID, item.Name, item.Mission, item.Schedule, string(actions),
		item.Enabled, time.Now().UTC(), item.LastRunAt, item.LastReport, item.LastQuiet)
	return err
}

func (s *AgentStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The runs go with it: a history whose agent no longer exists cannot be
	// read by anybody and cannot be deleted through any screen.
	if _, err := s.Store.db.ExecContext(ctx, `DELETE FROM agent_runs WHERE agent_id=$1`, id); err != nil {
		return err
	}
	_, err := s.Store.db.ExecContext(ctx, `DELETE FROM agents WHERE id=$1`, id)
	return err
}

func (s *AgentStore) GetByID(id string) (agent.Agent, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := s.Store.db.QueryRowContext(ctx, agentSelect+` WHERE id=$1`, id)
	item, err := scanAgent(row)
	if err == sql.ErrNoRows {
		return agent.Agent{}, false, nil
	}
	return item, err == nil, err
}

func (s *AgentStore) ListByOwner(ownerUserID string) ([]agent.Agent, error) {
	return s.list(agentSelect+` WHERE owner_user_id=$1 ORDER BY created_at`, ownerUserID)
}

func (s *AgentStore) ListAll() ([]agent.Agent, error) {
	return s.list(agentSelect + ` ORDER BY created_at`)
}

func (s *AgentStore) list(query string, args ...any) ([]agent.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []agent.Agent{}
	for rows.Next() {
		item, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const agentSelect = `SELECT id, owner_user_id, project_id, name, mission, schedule, actions_json,
	enabled, created_at, updated_at, last_run_at, last_report, last_quiet FROM agents`

func scanAgent(row rowScanner) (agent.Agent, error) {
	var item agent.Agent
	var actions string
	var lastRun sql.NullTime
	if err := row.Scan(&item.ID, &item.OwnerUserID, &item.ProjectID, &item.Name, &item.Mission,
		&item.Schedule, &actions, &item.Enabled, &item.CreatedAt, &item.UpdatedAt,
		&lastRun, &item.LastReport, &item.LastQuiet); err != nil {
		return agent.Agent{}, err
	}
	if lastRun.Valid {
		at := lastRun.Time.UTC()
		item.LastRunAt = &at
	}
	item.Actions = []string{}
	_ = json.Unmarshal([]byte(actions), &item.Actions)
	// Re-filtered on the way out, not only on the way in. A row edited by hand,
	// or written by a version that knew an action this one has withdrawn, must
	// not grant it.
	item.Actions = agent.NormaliseActions(item.Actions)
	return item, nil
}

func (s *AgentStore) AppendRun(run agent.Run) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actions, err := json.Marshal(run.Actions)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		INSERT INTO agent_runs (id, agent_id, report, quiet, actions_json, error, started_at, finished_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		run.ID, run.AgentID, run.Report, run.Quiet, string(actions), run.Error,
		run.StartedAt.UTC(), run.FinishedAt)
	return err
}

func (s *AgentStore) UpdateRun(run agent.Run) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actions, err := json.Marshal(run.Actions)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `
		UPDATE agent_runs SET report=$2, quiet=$3, actions_json=$4, error=$5, finished_at=$6 WHERE id=$1`,
		run.ID, run.Report, run.Quiet, string(actions), run.Error, run.FinishedAt)
	return err
}

func (s *AgentStore) ListRuns(agentID string, limit int) ([]agent.Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, `
		SELECT id, agent_id, report, quiet, actions_json, error, started_at, finished_at
		FROM agent_runs WHERE agent_id=$1 ORDER BY started_at DESC LIMIT $2`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []agent.Run{}
	for rows.Next() {
		var run agent.Run
		var actions string
		var finished sql.NullTime
		if err := rows.Scan(&run.ID, &run.AgentID, &run.Report, &run.Quiet, &actions,
			&run.Error, &run.StartedAt, &finished); err != nil {
			return nil, err
		}
		if finished.Valid {
			at := finished.Time.UTC()
			run.FinishedAt = &at
		}
		run.Actions = []string{}
		_ = json.Unmarshal([]byte(actions), &run.Actions)
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
