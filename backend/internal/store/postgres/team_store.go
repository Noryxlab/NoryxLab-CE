package postgres

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	storepkg "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

// TeamStore persists teams, their members and their grants.
type TeamStore struct{ *Store }

const teamSelect = `SELECT id, organization_id, name, description, created_at, updated_at FROM teams`

func scanTeam(row rowScanner) (team.Team, error) {
	var item team.Team
	if err := row.Scan(&item.ID, &item.OrganizationID, &item.Name,
		&item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return team.Team{}, err
	}
	return item, nil
}

// ListByOrganization returns an organization's teams, with how many people are
// in each.
//
// The count is joined rather than queried per row: a screen listing twelve
// teams would otherwise make thirteen round trips, and that shape of mistake
// only shows up once an installation has enough teams to be worth listing.
func (s *TeamStore) ListByOrganization(organizationID string) ([]team.Team, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.organization_id, t.name, t.description, t.created_at, t.updated_at,
		       COUNT(m.user_id)
		FROM teams t
		LEFT JOIN team_members m ON m.team_id = t.id
		WHERE t.organization_id = $1
		GROUP BY t.id
		ORDER BY lower(t.name)`, strings.TrimSpace(organizationID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []team.Team{}
	for rows.Next() {
		var item team.Team
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Description,
			&item.CreatedAt, &item.UpdatedAt, &item.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *TeamStore) GetByID(id string) (team.Team, bool, error) {
	item, err := scanTeam(s.db.QueryRow(teamSelect+` WHERE id=$1`, strings.TrimSpace(id)))
	if errors.Is(err, sql.ErrNoRows) {
		return team.Team{}, false, nil
	}
	if err != nil {
		return team.Team{}, false, err
	}
	return item, true, nil
}

func (s *TeamStore) Create(item team.Team) error {
	_, err := s.db.Exec(`
		INSERT INTO teams (id, organization_id, name, description, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		item.ID, item.OrganizationID, item.Name, item.Description, item.CreatedAt, item.UpdatedAt)
	return translateTeamNameConflict(err)
}

func (s *TeamStore) Update(item team.Team) error {
	result, err := s.db.Exec(`
		UPDATE teams SET name=$2, description=$3, updated_at=$4 WHERE id=$1`,
		item.ID, item.Name, item.Description, item.UpdatedAt)
	if err != nil {
		return translateTeamNameConflict(err)
	}
	// An update that matched nothing is a caller asking about something that
	// is not there, and reporting success would let an interface show a
	// rename that never happened.
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return team.ErrNotFound
	}
	return nil
}

// Delete removes the team, its membership and its grants together.
//
// In one transaction, because the three are one fact. A team deleted with its
// grants left behind would keep opening projects to a membership nobody can
// list any more - an orphan permission is the worst kind, since it is invisible
// to the screen that would have shown it.
func (s *TeamStore) Delete(id string) error {
	id = strings.TrimSpace(id)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range []string{
		`DELETE FROM access_team_roles WHERE team_id=$1`,
		`DELETE FROM team_members WHERE team_id=$1`,
		`DELETE FROM teams WHERE id=$1`,
	} {
		if _, err := tx.Exec(statement, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *TeamStore) ListMembers(teamID string) ([]team.Member, error) {
	rows, err := s.db.Query(`
		SELECT team_id, user_id, joined_at FROM team_members
		WHERE team_id=$1 ORDER BY joined_at`, strings.TrimSpace(teamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []team.Member{}
	for rows.Next() {
		var member team.Member
		if err := rows.Scan(&member.TeamID, &member.UserID, &member.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, member)
	}
	return out, rows.Err()
}

// AddMember is idempotent, and keeps the original joining date.
//
// Adding somebody who is already there is a normal thing for an interface to
// do - a double click, a re-imported list - and it must not look like a
// failure. It must not rewrite joined_at either: the date answers when this
// person gained what the team grants, and moving it forward would erase the
// only evidence an auditor has.
func (s *TeamStore) AddMember(teamID, userID string) error {
	_, err := s.db.Exec(`
		INSERT INTO team_members (team_id, user_id, joined_at)
		VALUES ($1,$2,$3) ON CONFLICT (team_id, user_id) DO NOTHING`,
		strings.TrimSpace(teamID), strings.TrimSpace(userID), time.Now().UTC())
	return err
}

func (s *TeamStore) RemoveMember(teamID, userID string) error {
	_, err := s.db.Exec(`DELETE FROM team_members WHERE team_id=$1 AND user_id=$2`,
		strings.TrimSpace(teamID), strings.TrimSpace(userID))
	return err
}

func (s *TeamStore) ListByUser(userID string) ([]team.Team, error) {
	rows, err := s.db.Query(teamSelect+`
		WHERE id IN (SELECT team_id FROM team_members WHERE user_id=$1)
		ORDER BY lower(name)`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []team.Team{}
	for rows.Next() {
		item, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SetProjectRole grants a role to a team, or revokes it when the role is empty.
//
// The same shape as the organization grant beside it, including that an empty
// role deletes rather than storing a blank: a row saying somebody has no role
// and no row at all are the same fact, and keeping both invites code that
// checks only one.
func (s *TeamStore) SetProjectRole(projectID, teamID string, role access.Role) error {
	projectID, teamID = strings.TrimSpace(projectID), strings.TrimSpace(teamID)
	if strings.TrimSpace(string(role)) == "" {
		_, err := s.db.Exec(
			`DELETE FROM access_team_roles WHERE project_id=$1 AND team_id=$2`, projectID, teamID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO access_team_roles (project_id, team_id, role)
		VALUES ($1,$2,$3)
		ON CONFLICT (project_id, team_id) DO UPDATE SET role = EXCLUDED.role`,
		projectID, teamID, string(role))
	return err
}

func (s *TeamStore) ListProjectRoles(projectID string) ([]storepkg.ProjectTeamRole, error) {
	rows, err := s.db.Query(`
		SELECT r.project_id, r.team_id, r.role, COALESCE(t.name, '')
		FROM access_team_roles r
		LEFT JOIN teams t ON t.id = r.team_id
		WHERE r.project_id=$1
		ORDER BY lower(COALESCE(t.name, r.team_id))`, strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectTeamRoles(rows)
}

func (s *TeamStore) ListProjectRolesForUser(projectID, userID string) ([]storepkg.ProjectTeamRole, error) {
	rows, err := s.db.Query(`
		SELECT r.project_id, r.team_id, r.role, COALESCE(t.name, '')
		FROM access_team_roles r
		JOIN team_members m ON m.team_id = r.team_id
		LEFT JOIN teams t ON t.id = r.team_id
		WHERE r.project_id=$1 AND m.user_id=$2`,
		strings.TrimSpace(projectID), strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectTeamRoles(rows)
}

func scanProjectTeamRoles(rows *sql.Rows) ([]storepkg.ProjectTeamRole, error) {
	out := []storepkg.ProjectTeamRole{}
	for rows.Next() {
		var grant storepkg.ProjectTeamRole
		var role string
		if err := rows.Scan(&grant.ProjectID, &grant.TeamID, &role, &grant.TeamName); err != nil {
			return nil, err
		}
		grant.Role = access.Role(role)
		out = append(out, grant)
	}
	return out, rows.Err()
}

// translateTeamNameConflict turns the unique index into the domain's own
// refusal, so a handler can answer 409 without knowing an index name.
func translateTeamNameConflict(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "teams_org_name_unique") {
		return team.ErrNameTaken
	}
	return err
}
