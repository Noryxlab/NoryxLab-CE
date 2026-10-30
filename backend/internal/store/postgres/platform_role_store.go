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

func (s *PlatformRoleStore) ListOverrides() ([]platformrole.Override, error) {
	rows, err := s.Store.db.Query(`SELECT role_key, added_json, removed_json, updated_at, updated_by FROM platform_role_overrides`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []platformrole.Override{}
	for rows.Next() {
		var item platformrole.Override
		var added, removed []byte
		if err := rows.Scan(&item.RoleKey, &added, &removed, &item.UpdatedAt, &item.UpdatedBy); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(added, &item.Added)
		_ = json.Unmarshal(removed, &item.Removed)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PlatformRoleStore) SetOverride(item platformrole.Override) error {
	added, _ := json.Marshal(item.Added)
	removed, _ := json.Marshal(item.Removed)
	now := time.Now().UTC()
	// Nothing changed means nothing stored: an installation back on the
	// platform's own answer should not carry a row saying so.
	if len(item.Added) == 0 && len(item.Removed) == 0 {
		_, err := s.Store.db.Exec(`DELETE FROM platform_role_overrides WHERE role_key=$1`, strings.ToLower(strings.TrimSpace(item.RoleKey)))
		return err
	}
	_, err := s.Store.db.Exec(`INSERT INTO platform_role_overrides (role_key, added_json, removed_json, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (role_key) DO UPDATE SET added_json=EXCLUDED.added_json, removed_json=EXCLUDED.removed_json, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		strings.ToLower(strings.TrimSpace(item.RoleKey)), added, removed, now, item.UpdatedBy)
	return err
}
