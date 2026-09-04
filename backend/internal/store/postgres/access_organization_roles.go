package postgres

import (
	"context"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

// Role grants made to an organization rather than to a person.
//
// Assigning one user at a time is workable for a team of five and a reason not
// to buy at fifty: an administrator adding a researcher has to remember every
// project that person should reach, and someone leaving has to be removed from
// each one. A grant to the organization is made once and follows its
// membership.

// SetProjectOrganizationRole grants a role, or revokes it when role is empty.
func (s *Store) SetProjectOrganizationRole(projectID, organizationID string, role access.Role) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if role == "" {
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM access_organization_roles WHERE project_id = $1 AND organization_id = $2`,
			projectID, organizationID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO access_organization_roles (project_id, organization_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, organization_id) DO UPDATE SET role = EXCLUDED.role`,
		projectID, organizationID, string(role))
	return err
}

func (s *Store) ListProjectOrganizationRoles(projectID string) ([]store.ProjectOrganizationRole, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, organization_id, role FROM access_organization_roles
		WHERE project_id = $1 ORDER BY organization_id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []store.ProjectOrganizationRole{}
	for rows.Next() {
		var item store.ProjectOrganizationRole
		var role string
		if err := rows.Scan(&item.ProjectID, &item.OrganizationID, &role); err != nil {
			return nil, err
		}
		item.Role = access.Role(role)
		out = append(out, item)
	}
	return out, rows.Err()
}
