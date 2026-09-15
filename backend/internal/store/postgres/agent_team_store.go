package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

type AgentTeamStore struct{ Store *Store }

const agentTeamSelect = `SELECT id, owner_user_id, project_id, name, purpose, created_at, updated_at
	FROM agent_teams`

func scanAgentTeam(row rowScanner) (agent.Team, error) {
	var item agent.Team
	if err := row.Scan(&item.ID, &item.OwnerUserID, &item.ProjectID, &item.Name,
		&item.Purpose, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return agent.Team{}, err
	}
	return item, nil
}

func (s *AgentTeamStore) ListByOwner(ownerUserID string) ([]agent.Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx,
		agentTeamSelect+` WHERE owner_user_id=$1 ORDER BY created_at`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []agent.Team{}
	for rows.Next() {
		item, err := scanAgentTeam(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *AgentTeamStore) GetByID(id string) (agent.Team, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	item, err := scanAgentTeam(s.Store.db.QueryRowContext(ctx, agentTeamSelect+` WHERE id=$1`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return agent.Team{}, false, nil
		}
		return agent.Team{}, false, err
	}
	return item, true, nil
}

func (s *AgentTeamStore) Create(item agent.Team) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Store.db.ExecContext(ctx, `
		INSERT INTO agent_teams (id, owner_user_id, project_id, name, purpose, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		item.ID, item.OwnerUserID, item.ProjectID, item.Name, item.Purpose,
		item.CreatedAt.UTC(), item.UpdatedAt.UTC())
	return err
}

func (s *AgentTeamStore) Update(item agent.Team) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Store.db.ExecContext(ctx, `
		UPDATE agent_teams SET project_id=$2, name=$3, purpose=$4, updated_at=$5 WHERE id=$1`,
		item.ID, item.ProjectID, item.Name, item.Purpose, time.Now().UTC())
	return err
}

func (s *AgentTeamStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The members are left where they are, holding a team that no longer
	// exists, which the domain already treats as working alone. Deleting them
	// here would destroy standing work because somebody tidied up a grouping.
	_, err := s.Store.db.ExecContext(ctx, `UPDATE agents SET team_id='' WHERE team_id=$1`, id)
	if err != nil {
		return err
	}
	_, err = s.Store.db.ExecContext(ctx, `DELETE FROM agent_teams WHERE id=$1`, id)
	return err
}

func (s *AgentTeamStore) ListMandates(teamID string) ([]agent.Mandate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, `
		SELECT id, team_id, lead_id, member_id, action, granted_by_user_id, created_at
		FROM agent_mandates WHERE team_id=$1 ORDER BY created_at`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []agent.Mandate{}
	for rows.Next() {
		var item agent.Mandate
		if err := rows.Scan(&item.ID, &item.TeamID, &item.LeadID, &item.MemberID,
			&item.Action, &item.GrantedByUserID, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *AgentTeamStore) CreateMandate(item agent.Mandate) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Writing the same line twice is not an error and must not create a second
	// row: the organisation is a set of statements, and a duplicate would make
	// "who may ask whom" answer differently depending on how it was counted.
	_, err := s.Store.db.ExecContext(ctx, `
		INSERT INTO agent_mandates (id, team_id, lead_id, member_id, action, granted_by_user_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (lead_id, member_id, action) DO NOTHING`,
		item.ID, item.TeamID, item.LeadID, item.MemberID, item.Action,
		item.GrantedByUserID, item.CreatedAt.UTC())
	return err
}

func (s *AgentTeamStore) DeleteMandate(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Store.db.ExecContext(ctx, `DELETE FROM agent_mandates WHERE id=$1`, id)
	return err
}

func (s *AgentTeamStore) ListAll() ([]agent.Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, agentTeamSelect+` ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []agent.Team{}
	for rows.Next() {
		item, err := scanAgentTeam(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *AgentTeamStore) ListAllMandates() ([]agent.Mandate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, `
		SELECT id, team_id, lead_id, member_id, action, granted_by_user_id, created_at
		FROM agent_mandates ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []agent.Mandate{}
	for rows.Next() {
		var item agent.Mandate
		if err := rows.Scan(&item.ID, &item.TeamID, &item.LeadID, &item.MemberID,
			&item.Action, &item.GrantedByUserID, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
