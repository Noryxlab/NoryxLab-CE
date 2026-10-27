package postgres

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
)

// PlatformRoleStore persists custom rows and per-person assignments.
type PlatformRoleStore struct{ Store *Store }

func (s *PlatformRoleStore) ListCustom() ([]platformrole.Role, error) {
	rows, err := s.Store.db.Query(`SELECT key, name, description, capabilities_json, created_at, updated_at FROM platform_roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []platformrole.Role{}
	for rows.Next() {
		var item platformrole.Role
		var capabilities []byte
		if err := rows.Scan(&item.Key, &item.Name, &item.Description, &capabilities, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(capabilities, &item.Capabilities)
		item.Assignable = true
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PlatformRoleStore) GetCustom(key string) (platformrole.Role, bool, error) {
	var item platformrole.Role
	var capabilities []byte
	err := s.Store.db.QueryRow(`SELECT key, name, description, capabilities_json, created_at, updated_at FROM platform_roles WHERE key=$1`,
		strings.ToLower(strings.TrimSpace(key))).Scan(&item.Key, &item.Name, &item.Description, &capabilities, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return platformrole.Role{}, false, nil
	}
	if err != nil {
		return platformrole.Role{}, false, err
	}
	_ = json.Unmarshal(capabilities, &item.Capabilities)
	item.Assignable = true
	return item, true, nil
}

func (s *PlatformRoleStore) UpsertCustom(role platformrole.Role) error {
	role.Normalise()
	capabilities, _ := json.Marshal(role.Capabilities)
	now := time.Now().UTC()
	_, err := s.Store.db.Exec(`INSERT INTO platform_roles (key, name, description, capabilities_json, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, capabilities_json=EXCLUDED.capabilities_json, updated_at=EXCLUDED.updated_at`,
		role.Key, role.Name, role.Description, capabilities, now, now)
	return err
}

func (s *PlatformRoleStore) DeleteCustom(key string) error {
	_, err := s.Store.db.Exec(`DELETE FROM platform_roles WHERE key=$1`, strings.ToLower(strings.TrimSpace(key)))
	return err
}

func (s *PlatformRoleStore) GetAssignment(userID string) (platformrole.Assignment, bool, error) {
	var item platformrole.Assignment
	err := s.Store.db.QueryRow(`SELECT user_id, role_key, assigned_by, assigned_at FROM user_platform_roles WHERE lower(user_id)=lower($1)`,
		strings.TrimSpace(userID)).Scan(&item.UserID, &item.RoleKey, &item.AssignedBy, &item.AssignedAt)
	if err == sql.ErrNoRows {
		return platformrole.Assignment{}, false, nil
	}
	if err != nil {
		return platformrole.Assignment{}, false, err
	}
	return item, true, nil
}

func (s *PlatformRoleStore) SetAssignment(item platformrole.Assignment) error {
	if item.AssignedAt.IsZero() {
		item.AssignedAt = time.Now().UTC()
	}
	_, err := s.Store.db.Exec(`INSERT INTO user_platform_roles (user_id, role_key, assigned_by, assigned_at) VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id) DO UPDATE SET role_key=EXCLUDED.role_key, assigned_by=EXCLUDED.assigned_by, assigned_at=EXCLUDED.assigned_at`,
		strings.TrimSpace(item.UserID), strings.ToLower(strings.TrimSpace(item.RoleKey)), strings.TrimSpace(item.AssignedBy), item.AssignedAt)
	return err
}

func (s *PlatformRoleStore) ClearAssignment(userID string) error {
	_, err := s.Store.db.Exec(`DELETE FROM user_platform_roles WHERE lower(user_id)=lower($1)`, strings.TrimSpace(userID))
	return err
}

func (s *PlatformRoleStore) ListAssignments() ([]platformrole.Assignment, error) {
	rows, err := s.Store.db.Query(`SELECT user_id, role_key, assigned_by, assigned_at FROM user_platform_roles ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []platformrole.Assignment{}
	for rows.Next() {
		var item platformrole.Assignment
		if err := rows.Scan(&item.UserID, &item.RoleKey, &item.AssignedBy, &item.AssignedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
