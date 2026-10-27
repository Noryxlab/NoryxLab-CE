package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
)

// OrganizationStore persists who administers what an organization owns.
type OrganizationStore struct{ Store *Store }

func (s *OrganizationStore) ListAdmins(organizationID string) ([]organization.Admin, error) {
	rows, err := s.Store.db.Query(
		`SELECT organization_id, subject_type, subject_id, created_at
		   FROM organization_admins WHERE organization_id=$1
		  ORDER BY subject_type, subject_id`,
		strings.TrimSpace(organizationID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []organization.Admin{}
	for rows.Next() {
		var item organization.Admin
		if err := rows.Scan(&item.OrganizationID, &item.SubjectType, &item.SubjectID, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *OrganizationStore) AddAdmin(item organization.Admin) error {
	item.Normalise()
	_, err := s.Store.db.Exec(
		`INSERT INTO organization_admins (organization_id, subject_type, subject_id, created_at)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (organization_id, subject_type, subject_id) DO NOTHING`,
		item.OrganizationID, item.SubjectType, item.SubjectID, item.CreatedAt)
	return err
}

func (s *OrganizationStore) RemoveAdmin(organizationID, subjectType, subjectID string) error {
	_, err := s.Store.db.Exec(
		`DELETE FROM organization_admins
		  WHERE organization_id=$1 AND subject_type=$2 AND subject_id=$3`,
		strings.TrimSpace(organizationID),
		strings.ToLower(strings.TrimSpace(subjectType)),
		strings.TrimSpace(subjectID))
	return err
}

// AdministeredBy answers the authorization path, so it asks once.
//
// The identifiers arrive from three sources that spell people differently -
// the directory's subject id, the username, the e-mail - and a team is named
// by its own identifier. Comparing them case-insensitively here is what stops
// a grant from silently not applying because one screen wrote "Malinet.C".
func (s *OrganizationStore) AdministeredBy(userIDs []string, teamIDs []string) ([]string, error) {
	conditions := []string{}
	args := []any{}
	for _, value := range nonEmptyLower(userIDs) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("(subject_type='user' AND lower(subject_id)=$%d)", len(args)))
	}
	for _, value := range nonEmptyLower(teamIDs) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("(subject_type='team' AND lower(subject_id)=$%d)", len(args)))
	}
	if len(conditions) == 0 {
		return nil, nil
	}
	rows, err := s.Store.db.Query(
		`SELECT DISTINCT organization_id FROM organization_admins WHERE `+strings.Join(conditions, " OR "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func nonEmptyLower(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *OrganizationStore) GetPolicy(organizationID string) (organization.Policy, bool, error) {
	var item organization.Policy
	err := s.Store.db.QueryRow(
		`SELECT organization_id, membership_grant, updated_at, updated_by
		   FROM organization_policies WHERE organization_id=$1`,
		strings.TrimSpace(organizationID)).Scan(&item.OrganizationID, &item.MembershipGrant, &item.UpdatedAt, &item.UpdatedBy)
	if err == sql.ErrNoRows {
		return organization.Policy{}, false, nil
	}
	if err != nil {
		return organization.Policy{}, false, err
	}
	return item, true, nil
}

func (s *OrganizationStore) SetPolicy(item organization.Policy) error {
	item.Normalise()
	_, err := s.Store.db.Exec(
		`INSERT INTO organization_policies (organization_id, membership_grant, updated_at, updated_by)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (organization_id) DO UPDATE SET
			membership_grant=EXCLUDED.membership_grant,
			updated_at=EXCLUDED.updated_at,
			updated_by=EXCLUDED.updated_by`,
		item.OrganizationID, item.MembershipGrant, item.UpdatedAt, item.UpdatedBy)
	return err
}
