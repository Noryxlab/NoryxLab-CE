package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/pod"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	storepkg "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	_ "github.com/lib/pq"
)

type Config struct {
	Host            string
	Port            string
	DBName          string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Store struct {
	db *sql.DB
}

func New(cfg Config) (*Store, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("database host is required")
	}
	if strings.TrimSpace(cfg.Port) == "" {
		cfg.Port = "5432"
	}
	if strings.TrimSpace(cfg.DBName) == "" {
		cfg.DBName = "noryx"
	}
	if strings.TrimSpace(cfg.User) == "" {
		cfg.User = "noryx"
	}
	if strings.TrimSpace(cfg.SSLMode) == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 20
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime <= 0 {
		cfg.ConnMaxLifetime = 30 * time.Minute
	}

	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.User,
		cfg.Password,
		cfg.SSLMode,
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			owner_type TEXT NOT NULL DEFAULT 'user',
			owner_id TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'user'`,
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS access_roles (
			project_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			PRIMARY KEY (project_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS builds (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			git_repository TEXT NOT NULL,
			git_ref TEXT NOT NULL,
			dockerfile_path TEXT NOT NULL,
			dockerfile_content TEXT NOT NULL DEFAULT '',
			context_path TEXT NOT NULL,
			destination_image TEXT NOT NULL,
			job_name TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`ALTER TABLE builds ADD COLUMN IF NOT EXISTS dockerfile_content TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS apps (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'app',
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			image TEXT NOT NULL,
			command_json JSONB,
			args_json JSONB,
			port INTEGER NOT NULL,
			pod_name TEXT NOT NULL,
			service_name TEXT NOT NULL,
			status TEXT NOT NULL,
			access_url TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`ALTER TABLE apps ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'app'`,
		`CREATE TABLE IF NOT EXISTS pods (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			pod_name TEXT NOT NULL,
			image TEXT NOT NULL,
			command_json JSONB,
			args_json JSONB,
			env_json JSONB,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			image TEXT NOT NULL,
			pod_name TEXT NOT NULL,
			service_name TEXT NOT NULL,
			pvc_name TEXT,
			pvc_class TEXT,
			pvc_size TEXT,
			pvc_mount_path TEXT,
			cpu TEXT,
			memory TEXT,
			status TEXT NOT NULL,
			access_url TEXT,
			access_token TEXT,
				created_at TIMESTAMPTZ NOT NULL
			)`,
		`CREATE TABLE IF NOT EXISTS jobs (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				name TEXT NOT NULL,
				image TEXT NOT NULL,
				command_json JSONB,
				args_json JSONB,
				job_name TEXT NOT NULL,
				status TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL
			)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			identity TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			occurred_at TIMESTAMPTZ NOT NULL,
			actor_user_id TEXT NOT NULL,
			actor_ip TEXT NOT NULL,
			actor_user_agent TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			outcome TEXT NOT NULL,
			error_code TEXT NOT NULL,
			details_json JSONB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_occurred_at ON audit_events (occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_actor_user_id ON audit_events (actor_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_project_id ON audit_events (project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events (action)`,
		`CREATE TABLE IF NOT EXISTS user_secrets (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			value_encrypted TEXT NOT NULL,
			expires_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (user_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS datasets (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			bucket TEXT NOT NULL,
			prefix TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'minio',
			classification TEXT NOT NULL DEFAULT 'non-hds',
			endpoint TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			credential_name TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'minio'`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS classification TEXT NOT NULL DEFAULT 'non-hds'`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS endpoint TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS region TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS credential_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'user'`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS credential_user_id TEXT NOT NULL DEFAULT ''`,
		`UPDATE datasets SET owner_id=owner_user_id WHERE owner_id=''`,
		`UPDATE datasets SET credential_user_id=owner_user_id WHERE credential_user_id=''`,
		`CREATE TABLE IF NOT EXISTS dataset_access (
			dataset_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (dataset_id, user_id)
		)`,
		`ALTER TABLE dataset_access ADD COLUMN IF NOT EXISTS subject_type TEXT NOT NULL DEFAULT 'user'`,
		`ALTER TABLE dataset_access ADD COLUMN IF NOT EXISTS subject_id TEXT NOT NULL DEFAULT ''`,
		`UPDATE dataset_access SET subject_id=user_id WHERE subject_id=''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_dataset_access_subject ON dataset_access (dataset_id, subject_type, subject_id)`,
		`CREATE TABLE IF NOT EXISTS datasources (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			database_name TEXT NOT NULL,
			username TEXT NOT NULL,
			password_secret TEXT NOT NULL,
			ssl_mode TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS repositories (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			default_ref TEXT NOT NULL,
			auth_secret_name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_datasets (
			project_id TEXT NOT NULL,
			dataset_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (project_id, dataset_id)
		)`,
		`CREATE TABLE IF NOT EXISTS project_repositories (
			project_id TEXT NOT NULL,
			repository_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (project_id, repository_id)
		)`,
		`CREATE TABLE IF NOT EXISTS project_datasources (
			project_id TEXT NOT NULL,
			datasource_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (project_id, datasource_id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (user_id, key)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE projects p
		SET owner_type='user',
			owner_id=(SELECT a.user_id FROM access_roles a WHERE a.project_id=p.id AND a.role='admin' ORDER BY a.user_id LIMIT 1)
		WHERE p.owner_id='' AND EXISTS (SELECT 1 FROM access_roles a WHERE a.project_id=p.id AND a.role='admin')`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE user_secrets ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL`); err != nil {
		return err
	}
	return nil
}

func (s *Store) List() ([]project.Project, error) {
	rows, err := s.db.Query(`SELECT id, name, owner_type, owner_id, created_at FROM projects ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []project.Project{}
	for rows.Next() {
		var p project.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.OwnerType, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Create(p project.Project) error {
	_, err := s.db.Exec(`INSERT INTO projects (id, name, owner_type, owner_id, created_at) VALUES ($1,$2,$3,$4,$5)`, p.ID, p.Name, p.OwnerType, p.OwnerID, p.CreatedAt)
	return err
}

func (s *Store) UpdateProjectOwner(projectID, ownerType, ownerID string) error {
	_, err := s.db.Exec(`UPDATE projects SET owner_type=$2, owner_id=$3 WHERE id=$1`, strings.TrimSpace(projectID), strings.TrimSpace(ownerType), strings.TrimSpace(ownerID))
	return err
}

func (s *Store) DeleteProject(projectID string) error {
	pid := strings.TrimSpace(projectID)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	statements := []string{
		`DELETE FROM access_roles WHERE project_id=$1`,
		`DELETE FROM project_datasets WHERE project_id=$1`,
		`DELETE FROM project_repositories WHERE project_id=$1`,
		`DELETE FROM project_datasources WHERE project_id=$1`,
		`DELETE FROM workspaces WHERE project_id=$1`,
		`DELETE FROM builds WHERE project_id=$1`,
		`DELETE FROM apps WHERE project_id=$1`,
		`DELETE FROM jobs WHERE project_id=$1`,
		`DELETE FROM pods WHERE project_id=$1`,
		`DELETE FROM projects WHERE id=$1`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(stmt, pid); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) SetRole(projectID, userID string, role access.Role) {
	_, err := s.db.Exec(`
		INSERT INTO access_roles (project_id, user_id, role) VALUES ($1,$2,$3)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, strings.TrimSpace(projectID), strings.TrimSpace(userID), string(role))
	if err != nil {
		log.Printf("warning: failed to persist access role: %v", err)
	}
}

func (s *Store) GetRole(projectID, userID string) (access.Role, bool) {
	var role string
	err := s.db.QueryRow(`SELECT role FROM access_roles WHERE project_id=$1 AND user_id=$2`, strings.TrimSpace(projectID), strings.TrimSpace(userID)).Scan(&role)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		log.Printf("warning: failed to read access role: %v", err)
		return "", false
	}
	return access.Role(role), true
}

func (s *Store) ListBuilds() ([]build.Build, error) {
	rows, err := s.db.Query(`SELECT id, project_id, git_repository, git_ref, dockerfile_path, dockerfile_content, context_path, destination_image, job_name, status, created_at FROM builds ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []build.Build{}
	for rows.Next() {
		var b build.Build
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.GitRepository, &b.GitRef, &b.DockerfilePath, &b.DockerfileContent, &b.ContextPath, &b.DestinationImage, &b.JobName, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) ListApps() ([]app.App, error) {
	rows, err := s.db.Query(`SELECT id, project_id, kind, name, slug, image, command_json, args_json, port, pod_name, service_name, status, access_url, created_at FROM apps ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.App{}
	for rows.Next() {
		var item app.App
		var commandJSON, argsJSON []byte
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Kind, &item.Name, &item.Slug, &item.Image, &commandJSON, &argsJSON, &item.Port, &item.PodName, &item.ServiceName, &item.Status, &item.AccessURL, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(commandJSON) > 0 {
			_ = json.Unmarshal(commandJSON, &item.Command)
		}
		if len(argsJSON) > 0 {
			_ = json.Unmarshal(argsJSON, &item.Args)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetAppByID(id string) (app.App, bool, error) {
	var item app.App
	var commandJSON, argsJSON []byte
	err := s.db.QueryRow(`SELECT id, project_id, kind, name, slug, image, command_json, args_json, port, pod_name, service_name, status, access_url, created_at FROM apps WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&item.ID,
		&item.ProjectID,
		&item.Kind,
		&item.Name,
		&item.Slug,
		&item.Image,
		&commandJSON,
		&argsJSON,
		&item.Port,
		&item.PodName,
		&item.ServiceName,
		&item.Status,
		&item.AccessURL,
		&item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return app.App{}, false, nil
	}
	if err != nil {
		return app.App{}, false, err
	}
	if len(commandJSON) > 0 {
		_ = json.Unmarshal(commandJSON, &item.Command)
	}
	if len(argsJSON) > 0 {
		_ = json.Unmarshal(argsJSON, &item.Args)
	}
	return item, true, nil
}

func (s *Store) GetAppBySlug(slug string) (app.App, bool, error) {
	var item app.App
	var commandJSON, argsJSON []byte
	err := s.db.QueryRow(`SELECT id, project_id, kind, name, slug, image, command_json, args_json, port, pod_name, service_name, status, access_url, created_at FROM apps WHERE slug=$1`, strings.TrimSpace(strings.ToLower(slug))).Scan(
		&item.ID,
		&item.ProjectID,
		&item.Kind,
		&item.Name,
		&item.Slug,
		&item.Image,
		&commandJSON,
		&argsJSON,
		&item.Port,
		&item.PodName,
		&item.ServiceName,
		&item.Status,
		&item.AccessURL,
		&item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return app.App{}, false, nil
	}
	if err != nil {
		return app.App{}, false, err
	}
	if len(commandJSON) > 0 {
		_ = json.Unmarshal(commandJSON, &item.Command)
	}
	if len(argsJSON) > 0 {
		_ = json.Unmarshal(argsJSON, &item.Args)
	}
	return item, true, nil
}

func (s *Store) CreateApp(item app.App) error {
	commandJSON, _ := json.Marshal(item.Command)
	argsJSON, _ := json.Marshal(item.Args)
	_, err := s.db.Exec(`INSERT INTO apps (id, project_id, kind, name, slug, image, command_json, args_json, port, pod_name, service_name, status, access_url, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		item.ID,
		item.ProjectID,
		item.Kind,
		item.Name,
		item.Slug,
		item.Image,
		commandJSON,
		argsJSON,
		item.Port,
		item.PodName,
		item.ServiceName,
		item.Status,
		item.AccessURL,
		item.CreatedAt,
	)
	return err
}

func (s *Store) UpsertApp(item app.App) error {
	commandJSON, _ := json.Marshal(item.Command)
	argsJSON, _ := json.Marshal(item.Args)
	_, err := s.db.Exec(`
		INSERT INTO apps (id, project_id, kind, name, slug, image, command_json, args_json, port, pod_name, service_name, status, access_url, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			project_id=EXCLUDED.project_id,
			kind=EXCLUDED.kind,
			name=EXCLUDED.name,
			slug=EXCLUDED.slug,
			image=EXCLUDED.image,
			command_json=EXCLUDED.command_json,
			args_json=EXCLUDED.args_json,
			port=EXCLUDED.port,
			pod_name=EXCLUDED.pod_name,
			service_name=EXCLUDED.service_name,
			status=EXCLUDED.status,
			access_url=EXCLUDED.access_url
	`,
		item.ID,
		item.ProjectID,
		item.Kind,
		item.Name,
		item.Slug,
		item.Image,
		commandJSON,
		argsJSON,
		item.Port,
		item.PodName,
		item.ServiceName,
		item.Status,
		item.AccessURL,
		item.CreatedAt,
	)
	return err
}

func (s *Store) DeleteApp(id string) error {
	_, err := s.db.Exec(`DELETE FROM apps WHERE id=$1`, strings.TrimSpace(id))
	return err
}

func (s *Store) GetBuildByID(id string) (build.Build, bool, error) {
	var b build.Build
	err := s.db.QueryRow(`SELECT id, project_id, git_repository, git_ref, dockerfile_path, dockerfile_content, context_path, destination_image, job_name, status, created_at FROM builds WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&b.ID,
		&b.ProjectID,
		&b.GitRepository,
		&b.GitRef,
		&b.DockerfilePath,
		&b.DockerfileContent,
		&b.ContextPath,
		&b.DestinationImage,
		&b.JobName,
		&b.Status,
		&b.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return build.Build{}, false, nil
	}
	if err != nil {
		return build.Build{}, false, err
	}
	return b, true, nil
}

func (s *Store) CreateBuild(b build.Build) error {
	_, err := s.db.Exec(`INSERT INTO builds (id, project_id, git_repository, git_ref, dockerfile_path, dockerfile_content, context_path, destination_image, job_name, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		b.ID,
		b.ProjectID,
		b.GitRepository,
		b.GitRef,
		b.DockerfilePath,
		b.DockerfileContent,
		b.ContextPath,
		b.DestinationImage,
		b.JobName,
		b.Status,
		b.CreatedAt,
	)
	return err
}

func (s *Store) UpsertBuild(b build.Build) error {
	_, err := s.db.Exec(`
		INSERT INTO builds (id, project_id, git_repository, git_ref, dockerfile_path, dockerfile_content, context_path, destination_image, job_name, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			project_id=EXCLUDED.project_id,
			git_repository=EXCLUDED.git_repository,
			git_ref=EXCLUDED.git_ref,
			dockerfile_path=EXCLUDED.dockerfile_path,
			dockerfile_content=EXCLUDED.dockerfile_content,
			context_path=EXCLUDED.context_path,
			destination_image=EXCLUDED.destination_image,
			job_name=EXCLUDED.job_name,
			status=EXCLUDED.status
	`,
		b.ID,
		b.ProjectID,
		b.GitRepository,
		b.GitRef,
		b.DockerfilePath,
		b.DockerfileContent,
		b.ContextPath,
		b.DestinationImage,
		b.JobName,
		b.Status,
		b.CreatedAt,
	)
	return err
}

func (s *Store) DeleteBuild(id string) error {
	_, err := s.db.Exec(`DELETE FROM builds WHERE id=$1`, strings.TrimSpace(id))
	return err
}

func (s *Store) ListJobs() ([]job.Job, error) {
	rows, err := s.db.Query(`SELECT id, project_id, name, image, command_json, args_json, job_name, status, created_at FROM jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []job.Job{}
	for rows.Next() {
		var item job.Job
		var commandJSON, argsJSON []byte
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Image, &commandJSON, &argsJSON, &item.JobName, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(commandJSON) > 0 {
			_ = json.Unmarshal(commandJSON, &item.Command)
		}
		if len(argsJSON) > 0 {
			_ = json.Unmarshal(argsJSON, &item.Args)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetJobByID(id string) (job.Job, bool, error) {
	var item job.Job
	var commandJSON, argsJSON []byte
	err := s.db.QueryRow(`SELECT id, project_id, name, image, command_json, args_json, job_name, status, created_at FROM jobs WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&item.ID,
		&item.ProjectID,
		&item.Name,
		&item.Image,
		&commandJSON,
		&argsJSON,
		&item.JobName,
		&item.Status,
		&item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return job.Job{}, false, nil
	}
	if err != nil {
		return job.Job{}, false, err
	}
	if len(commandJSON) > 0 {
		_ = json.Unmarshal(commandJSON, &item.Command)
	}
	if len(argsJSON) > 0 {
		_ = json.Unmarshal(argsJSON, &item.Args)
	}
	return item, true, nil
}

func (s *Store) CreateJob(item job.Job) error {
	commandJSON, _ := json.Marshal(item.Command)
	argsJSON, _ := json.Marshal(item.Args)
	_, err := s.db.Exec(`INSERT INTO jobs (id, project_id, name, image, command_json, args_json, job_name, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		item.ID,
		item.ProjectID,
		item.Name,
		item.Image,
		commandJSON,
		argsJSON,
		item.JobName,
		item.Status,
		item.CreatedAt,
	)
	return err
}

func (s *Store) UpsertJob(item job.Job) error {
	commandJSON, _ := json.Marshal(item.Command)
	argsJSON, _ := json.Marshal(item.Args)
	_, err := s.db.Exec(`
		INSERT INTO jobs (id, project_id, name, image, command_json, args_json, job_name, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			project_id=EXCLUDED.project_id,
			name=EXCLUDED.name,
			image=EXCLUDED.image,
			command_json=EXCLUDED.command_json,
			args_json=EXCLUDED.args_json,
			job_name=EXCLUDED.job_name,
			status=EXCLUDED.status
	`,
		item.ID,
		item.ProjectID,
		item.Name,
		item.Image,
		commandJSON,
		argsJSON,
		item.JobName,
		item.Status,
		item.CreatedAt,
	)
	return err
}

func (s *Store) DeleteJob(id string) error {
	_, err := s.db.Exec(`DELETE FROM jobs WHERE id=$1`, strings.TrimSpace(id))
	return err
}

func (s *Store) ListPods() ([]pod.Launch, error) {
	rows, err := s.db.Query(`SELECT id, project_id, pod_name, image, command_json, args_json, env_json, status, created_at FROM pods ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pod.Launch{}
	for rows.Next() {
		var item pod.Launch
		var commandJSON, argsJSON, envJSON []byte
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.PodName, &item.Image, &commandJSON, &argsJSON, &envJSON, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Command = unmarshalStringSlice(commandJSON)
		item.Args = unmarshalStringSlice(argsJSON)
		item.Env = unmarshalStringMap(envJSON)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreatePod(p pod.Launch) error {
	_, err := s.db.Exec(`INSERT INTO pods (id, project_id, pod_name, image, command_json, args_json, env_json, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		p.ID,
		p.ProjectID,
		p.PodName,
		p.Image,
		marshalJSON(p.Command),
		marshalJSON(p.Args),
		marshalJSON(p.Env),
		p.Status,
		p.CreatedAt,
	)
	return err
}

func (s *Store) ListWorkspaces() ([]workspace.Workspace, error) {
	rows, err := s.db.Query(`SELECT id, project_id, kind, name, image, pod_name, service_name, pvc_name, pvc_class, pvc_size, pvc_mount_path, cpu, memory, status, access_url, access_token, created_at FROM workspaces ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []workspace.Workspace{}
	for rows.Next() {
		var w workspace.Workspace
		if err := rows.Scan(
			&w.ID,
			&w.ProjectID,
			&w.Kind,
			&w.Name,
			&w.Image,
			&w.PodName,
			&w.ServiceName,
			&w.PVCName,
			&w.PVCClass,
			&w.PVCSize,
			&w.PVCMountPath,
			&w.CPU,
			&w.Memory,
			&w.Status,
			&w.AccessURL,
			&w.AccessToken,
			&w.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) GetWorkspaceByID(id string) (workspace.Workspace, bool, error) {
	var w workspace.Workspace
	err := s.db.QueryRow(`SELECT id, project_id, kind, name, image, pod_name, service_name, pvc_name, pvc_class, pvc_size, pvc_mount_path, cpu, memory, status, access_url, access_token, created_at FROM workspaces WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&w.ID,
		&w.ProjectID,
		&w.Kind,
		&w.Name,
		&w.Image,
		&w.PodName,
		&w.ServiceName,
		&w.PVCName,
		&w.PVCClass,
		&w.PVCSize,
		&w.PVCMountPath,
		&w.CPU,
		&w.Memory,
		&w.Status,
		&w.AccessURL,
		&w.AccessToken,
		&w.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return workspace.Workspace{}, false, nil
	}
	if err != nil {
		return workspace.Workspace{}, false, err
	}
	return w, true, nil
}

func (s *Store) CreateWorkspace(w workspace.Workspace) error {
	_, err := s.db.Exec(`INSERT INTO workspaces (id, project_id, kind, name, image, pod_name, service_name, pvc_name, pvc_class, pvc_size, pvc_mount_path, cpu, memory, status, access_url, access_token, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		w.ID,
		w.ProjectID,
		w.Kind,
		w.Name,
		w.Image,
		w.PodName,
		w.ServiceName,
		w.PVCName,
		w.PVCClass,
		w.PVCSize,
		w.PVCMountPath,
		w.CPU,
		w.Memory,
		w.Status,
		w.AccessURL,
		w.AccessToken,
		w.CreatedAt,
	)
	return err
}

func (s *Store) DeleteWorkspace(id string) error {
	_, err := s.db.Exec(`DELETE FROM workspaces WHERE id=$1`, strings.TrimSpace(id))
	return err
}

func (s *Store) CreateSession(item session.Session) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token, identity, expires_at) VALUES ($1,$2,$3) ON CONFLICT (token) DO UPDATE SET identity=EXCLUDED.identity, expires_at=EXCLUDED.expires_at`, item.Token, item.Identity, item.ExpiresAt)
	return err
}

func (s *Store) GetSession(token string) (session.Session, bool, error) {
	var item session.Session
	err := s.db.QueryRow(`SELECT token, identity, expires_at FROM sessions WHERE token=$1`, strings.TrimSpace(token)).Scan(&item.Token, &item.Identity, &item.ExpiresAt)
	if err == sql.ErrNoRows {
		return session.Session{}, false, nil
	}
	if err != nil {
		return session.Session{}, false, err
	}
	return item, true, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token=$1`, strings.TrimSpace(token))
	return err
}

func (s *Store) CreateAuditEvent(event audit.Event) error {
	detailsJSON, _ := json.Marshal(event.Details)
	_, err := s.db.Exec(`INSERT INTO audit_events (id, occurred_at, actor_user_id, actor_ip, actor_user_agent, action, resource_type, resource_id, project_id, outcome, error_code, details_json) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		event.ID,
		event.OccurredAt,
		event.ActorUserID,
		event.ActorIP,
		event.ActorAgent,
		event.Action,
		event.ResourceType,
		event.ResourceID,
		event.ProjectID,
		event.Outcome,
		event.ErrorCode,
		detailsJSON,
	)
	return err
}

func (s *Store) ListAuditEvents(filter storepkg.AuditFilter) ([]audit.Event, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	clauses := []string{"1=1"}
	args := []any{}
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Since != nil {
		clauses = append(clauses, "occurred_at >= "+nextArg(*filter.Since))
	}
	if filter.Until != nil {
		clauses = append(clauses, "occurred_at <= "+nextArg(*filter.Until))
	}
	if v := strings.TrimSpace(filter.Action); v != "" {
		clauses = append(clauses, "action = "+nextArg(v))
	}
	if v := strings.TrimSpace(filter.ActorUserID); v != "" {
		clauses = append(clauses, "actor_user_id = "+nextArg(v))
	}
	if v := strings.TrimSpace(filter.ProjectID); v != "" {
		clauses = append(clauses, "project_id = "+nextArg(v))
	}
	query := `SELECT id, occurred_at, actor_user_id, actor_ip, actor_user_agent, action, resource_type, resource_id, project_id, outcome, error_code, details_json
		FROM audit_events
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY occurred_at DESC
		LIMIT ` + nextArg(limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []audit.Event{}
	for rows.Next() {
		var item audit.Event
		var detailsJSON []byte
		if err := rows.Scan(
			&item.ID,
			&item.OccurredAt,
			&item.ActorUserID,
			&item.ActorIP,
			&item.ActorAgent,
			&item.Action,
			&item.ResourceType,
			&item.ResourceID,
			&item.ProjectID,
			&item.Outcome,
			&item.ErrorCode,
			&detailsJSON,
		); err != nil {
			return nil, err
		}
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &item.Details)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListByUser(userID string) ([]secret.Secret, error) {
	rows, err := s.db.Query(`SELECT id, user_id, name, type, value_encrypted, expires_at, created_at, updated_at FROM user_secrets WHERE user_id=$1 ORDER BY updated_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []secret.Secret{}
	for rows.Next() {
		var item secret.Secret
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Type, &item.ValueEncrypted, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetByName(userID, name string) (secret.Secret, bool, error) {
	var item secret.Secret
	err := s.db.QueryRow(`SELECT id, user_id, name, type, value_encrypted, expires_at, created_at, updated_at FROM user_secrets WHERE user_id=$1 AND name=$2`, strings.TrimSpace(userID), strings.TrimSpace(name)).Scan(
		&item.ID,
		&item.UserID,
		&item.Name,
		&item.Type,
		&item.ValueEncrypted,
		&item.ExpiresAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return secret.Secret{}, false, nil
	}
	if err != nil {
		return secret.Secret{}, false, err
	}
	return item, true, nil
}

func (s *Store) Upsert(item secret.Secret) error {
	_, err := s.db.Exec(`
		INSERT INTO user_secrets (id, user_id, name, type, value_encrypted, expires_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (user_id, name) DO UPDATE SET
			type=EXCLUDED.type,
			value_encrypted=EXCLUDED.value_encrypted,
			expires_at=EXCLUDED.expires_at,
			updated_at=EXCLUDED.updated_at
	`, item.ID, item.UserID, item.Name, item.Type, item.ValueEncrypted, item.ExpiresAt, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) Delete(userID, name string) error {
	_, err := s.db.Exec(`DELETE FROM user_secrets WHERE user_id=$1 AND name=$2`, strings.TrimSpace(userID), strings.TrimSpace(name))
	return err
}

func (s *Store) ListDatasetsBySubjects(subjects []dataset.Subject) ([]dataset.Dataset, error) {
	conditions := []string{}
	args := []any{}
	for _, subject := range subjects {
		if strings.TrimSpace(subject.ID) == "" {
			continue
		}
		args = append(args, strings.TrimSpace(subject.Type), strings.TrimSpace(subject.ID))
		n := len(args)
		conditions = append(conditions, fmt.Sprintf("(d.owner_type=$%d AND d.owner_id=$%d) OR EXISTS (SELECT 1 FROM dataset_access a WHERE a.dataset_id=d.id AND a.subject_type=$%d AND a.subject_id=$%d)", n-1, n, n-1, n))
	}
	if len(conditions) == 0 {
		return []dataset.Dataset{}, nil
	}
	ownerConditions := []string{}
	writerConditions := []string{}
	for i := 0; i < len(args); i += 2 {
		ownerConditions = append(ownerConditions, fmt.Sprintf("(d.owner_type=$%d AND d.owner_id=$%d)", i+1, i+2))
		writerConditions = append(writerConditions, fmt.Sprintf("EXISTS (SELECT 1 FROM dataset_access a WHERE a.dataset_id=d.id AND a.subject_type=$%d AND a.subject_id=$%d AND a.role='writer')", i+1, i+2))
	}
	query := `SELECT d.id, d.owner_user_id, d.owner_type, d.owner_id, d.name, d.description, d.bucket, d.prefix, d.provider, d.classification, d.endpoint, d.region, d.credential_name, d.credential_user_id, d.created_at, d.updated_at,
		CASE WHEN ` + strings.Join(ownerConditions, " OR ") + ` THEN 'owner' WHEN ` + strings.Join(writerConditions, " OR ") + ` THEN 'writer' ELSE 'reader' END
		FROM datasets d
		WHERE ` + strings.Join(conditions, " OR ") + ` ORDER BY d.updated_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dataset.Dataset{}
	for rows.Next() {
		var item dataset.Dataset
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.OwnerType, &item.OwnerID, &item.Name, &item.Description, &item.Bucket, &item.Prefix, &item.Provider, &item.Classification, &item.Endpoint, &item.Region, &item.CredentialName, &item.CredentialUserID, &item.CreatedAt, &item.UpdatedAt, &item.AccessRole); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListAllDatasets() ([]dataset.Dataset, error) {
	rows, err := s.db.Query(`SELECT id, owner_user_id, owner_type, owner_id, name, description, bucket, prefix, provider, classification, endpoint, region, credential_name, credential_user_id, created_at, updated_at FROM datasets ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dataset.Dataset{}
	for rows.Next() {
		var item dataset.Dataset
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.OwnerType, &item.OwnerID, &item.Name, &item.Description, &item.Bucket, &item.Prefix, &item.Provider, &item.Classification, &item.Endpoint, &item.Region, &item.CredentialName, &item.CredentialUserID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetDatasetByID(id string) (dataset.Dataset, bool, error) {
	var item dataset.Dataset
	err := s.db.QueryRow(`SELECT id, owner_user_id, owner_type, owner_id, name, description, bucket, prefix, provider, classification, endpoint, region, credential_name, credential_user_id, created_at, updated_at FROM datasets WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&item.ID,
		&item.OwnerUserID,
		&item.OwnerType,
		&item.OwnerID,
		&item.Name,
		&item.Description,
		&item.Bucket,
		&item.Prefix,
		&item.Provider,
		&item.Classification,
		&item.Endpoint,
		&item.Region,
		&item.CredentialName,
		&item.CredentialUserID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return dataset.Dataset{}, false, nil
	}
	if err != nil {
		return dataset.Dataset{}, false, err
	}
	return item, true, nil
}

func (s *Store) CreateDataset(item dataset.Dataset) error {
	_, err := s.db.Exec(`INSERT INTO datasets (id, owner_user_id, owner_type, owner_id, name, description, bucket, prefix, provider, classification, endpoint, region, credential_name, credential_user_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		item.ID,
		item.OwnerUserID,
		item.OwnerType,
		item.OwnerID,
		item.Name,
		item.Description,
		item.Bucket,
		item.Prefix,
		item.Provider,
		item.Classification,
		item.Endpoint,
		item.Region,
		item.CredentialName,
		item.CredentialUserID,
		item.CreatedAt,
		item.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateDatasetOwner(datasetID, ownerType, ownerID string) error {
	_, err := s.db.Exec(`UPDATE datasets SET owner_type=$2, owner_id=$3, owner_user_id=CASE WHEN $2='user' THEN $3 ELSE owner_user_id END, updated_at=$4 WHERE id=$1`, strings.TrimSpace(datasetID), strings.TrimSpace(ownerType), strings.TrimSpace(ownerID), time.Now().UTC())
	return err
}

func (s *Store) DeleteDataset(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM project_datasets WHERE dataset_id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM dataset_access WHERE dataset_id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM datasets WHERE id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListDatasetAccess(datasetID string) ([]dataset.Access, error) {
	rows, err := s.db.Query(`SELECT dataset_id, user_id, subject_type, subject_id, role, created_at, updated_at FROM dataset_access WHERE dataset_id=$1 ORDER BY subject_type, subject_id`, strings.TrimSpace(datasetID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dataset.Access{}
	for rows.Next() {
		var item dataset.Access
		if err := rows.Scan(&item.DatasetID, &item.UserID, &item.SubjectType, &item.SubjectID, &item.Role, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetDatasetAccess(datasetID, subjectType, subjectID string) (dataset.Access, bool, error) {
	var item dataset.Access
	err := s.db.QueryRow(`SELECT dataset_id, user_id, subject_type, subject_id, role, created_at, updated_at FROM dataset_access WHERE dataset_id=$1 AND subject_type=$2 AND subject_id=$3`, strings.TrimSpace(datasetID), strings.TrimSpace(subjectType), strings.TrimSpace(subjectID)).Scan(&item.DatasetID, &item.UserID, &item.SubjectType, &item.SubjectID, &item.Role, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return dataset.Access{}, false, nil
	}
	return item, err == nil, err
}

func (s *Store) SetDatasetAccess(item dataset.Access) error {
	_, err := s.db.Exec(`INSERT INTO dataset_access (dataset_id,user_id,subject_type,subject_id,role,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (dataset_id,subject_type,subject_id) DO UPDATE SET role=EXCLUDED.role,updated_at=EXCLUDED.updated_at`, item.DatasetID, item.SubjectID, item.SubjectType, item.SubjectID, item.Role, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) DeleteDatasetAccess(datasetID, subjectType, subjectID string) error {
	_, err := s.db.Exec(`DELETE FROM dataset_access WHERE dataset_id=$1 AND subject_type=$2 AND subject_id=$3`, strings.TrimSpace(datasetID), strings.TrimSpace(subjectType), strings.TrimSpace(subjectID))
	return err
}

func (s *Store) ListDatasourcesByUser(userID string) ([]datasource.Datasource, error) {
	rows, err := s.db.Query(`SELECT id, owner_user_id, name, type, host, port, database_name, username, password_secret, ssl_mode, created_at, updated_at FROM datasources WHERE owner_user_id=$1 ORDER BY updated_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []datasource.Datasource{}
	for rows.Next() {
		var item datasource.Datasource
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.Name, &item.Type, &item.Host, &item.Port, &item.Database, &item.Username, &item.PasswordSecret, &item.SSLMode, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetDatasourceByID(id string) (datasource.Datasource, bool, error) {
	var item datasource.Datasource
	err := s.db.QueryRow(`SELECT id, owner_user_id, name, type, host, port, database_name, username, password_secret, ssl_mode, created_at, updated_at FROM datasources WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&item.ID, &item.OwnerUserID, &item.Name, &item.Type, &item.Host, &item.Port, &item.Database, &item.Username, &item.PasswordSecret, &item.SSLMode, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return datasource.Datasource{}, false, nil
	}
	if err != nil {
		return datasource.Datasource{}, false, err
	}
	return item, true, nil
}

func (s *Store) CreateDatasource(item datasource.Datasource) error {
	_, err := s.db.Exec(`INSERT INTO datasources (id, owner_user_id, name, type, host, port, database_name, username, password_secret, ssl_mode, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		item.ID, item.OwnerUserID, item.Name, item.Type, item.Host, item.Port, item.Database, item.Username, item.PasswordSecret, item.SSLMode, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func (s *Store) DeleteDatasource(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM project_datasources WHERE datasource_id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM datasources WHERE id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListRepositoriesByUser(userID string) ([]repository.Repository, error) {
	rows, err := s.db.Query(`SELECT id, owner_user_id, name, url, default_ref, auth_secret_name, created_at, updated_at FROM repositories WHERE owner_user_id=$1 ORDER BY updated_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []repository.Repository{}
	for rows.Next() {
		var item repository.Repository
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.Name, &item.URL, &item.DefaultRef, &item.AuthSecretName, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetRepositoryByID(id string) (repository.Repository, bool, error) {
	var item repository.Repository
	err := s.db.QueryRow(`SELECT id, owner_user_id, name, url, default_ref, auth_secret_name, created_at, updated_at FROM repositories WHERE id=$1`, strings.TrimSpace(id)).Scan(
		&item.ID,
		&item.OwnerUserID,
		&item.Name,
		&item.URL,
		&item.DefaultRef,
		&item.AuthSecretName,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return repository.Repository{}, false, nil
	}
	if err != nil {
		return repository.Repository{}, false, err
	}
	return item, true, nil
}

func (s *Store) CreateRepository(item repository.Repository) error {
	_, err := s.db.Exec(`INSERT INTO repositories (id, owner_user_id, name, url, default_ref, auth_secret_name, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		item.ID,
		item.OwnerUserID,
		item.Name,
		item.URL,
		item.DefaultRef,
		item.AuthSecretName,
		item.CreatedAt,
		item.UpdatedAt,
	)
	return err
}

func (s *Store) DeleteRepository(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM project_repositories WHERE repository_id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM repositories WHERE id=$1`, strings.TrimSpace(id)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AttachDataset(projectID, datasetID string) error {
	_, err := s.db.Exec(`INSERT INTO project_datasets (project_id, dataset_id, created_at) VALUES ($1,$2,$3) ON CONFLICT (project_id, dataset_id) DO NOTHING`, strings.TrimSpace(projectID), strings.TrimSpace(datasetID), time.Now().UTC())
	return err
}

func (s *Store) DetachDataset(projectID, datasetID string) error {
	_, err := s.db.Exec(`DELETE FROM project_datasets WHERE project_id=$1 AND dataset_id=$2`, strings.TrimSpace(projectID), strings.TrimSpace(datasetID))
	return err
}

func (s *Store) ListProjectDatasetIDs(projectID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT dataset_id FROM project_datasets WHERE project_id=$1 ORDER BY created_at ASC`, strings.TrimSpace(projectID))
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

func (s *Store) AttachRepository(projectID, repositoryID string) error {
	_, err := s.db.Exec(`INSERT INTO project_repositories (project_id, repository_id, created_at) VALUES ($1,$2,$3) ON CONFLICT (project_id, repository_id) DO NOTHING`, strings.TrimSpace(projectID), strings.TrimSpace(repositoryID), time.Now().UTC())
	return err
}

func (s *Store) DetachRepository(projectID, repositoryID string) error {
	_, err := s.db.Exec(`DELETE FROM project_repositories WHERE project_id=$1 AND repository_id=$2`, strings.TrimSpace(projectID), strings.TrimSpace(repositoryID))
	return err
}

func (s *Store) ListProjectRepositoryIDs(projectID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT repository_id FROM project_repositories WHERE project_id=$1 ORDER BY created_at ASC`, strings.TrimSpace(projectID))
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

func (s *Store) AttachDatasource(projectID, datasourceID string) error {
	_, err := s.db.Exec(`INSERT INTO project_datasources (project_id, datasource_id, created_at) VALUES ($1,$2,$3) ON CONFLICT (project_id, datasource_id) DO NOTHING`, strings.TrimSpace(projectID), strings.TrimSpace(datasourceID), time.Now().UTC())
	return err
}

func (s *Store) DetachDatasource(projectID, datasourceID string) error {
	_, err := s.db.Exec(`DELETE FROM project_datasources WHERE project_id=$1 AND datasource_id=$2`, strings.TrimSpace(projectID), strings.TrimSpace(datasourceID))
	return err
}

func (s *Store) ListProjectDatasourceIDs(projectID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT datasource_id FROM project_datasources WHERE project_id=$1 ORDER BY created_at ASC`, strings.TrimSpace(projectID))
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

func (s *Store) GetUserPreference(userID, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM user_preferences WHERE user_id=$1 AND key=$2`,
		strings.TrimSpace(userID),
		strings.TrimSpace(key),
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) SetUserPreference(userID, key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_preferences (user_id, key, value, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id, key) DO UPDATE SET
			value=EXCLUDED.value,
			updated_at=EXCLUDED.updated_at
	`,
		strings.TrimSpace(userID),
		strings.TrimSpace(key),
		strings.TrimSpace(value),
		time.Now().UTC(),
	)
	return err
}

func marshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}

func unmarshalStringSlice(raw []byte) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func unmarshalStringMap(raw []byte) map[string]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
