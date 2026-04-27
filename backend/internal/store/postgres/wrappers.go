package postgres

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/pod"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
)

type ProjectStore struct{ *Store }
type AccessStore struct{ *Store }

type BuildStore struct{ *Store }

func (s *BuildStore) List() ([]build.Build, error)                 { return s.Store.ListBuilds() }
func (s *BuildStore) GetByID(id string) (build.Build, bool, error) { return s.Store.GetBuildByID(id) }
func (s *BuildStore) Create(b build.Build) error                   { return s.Store.CreateBuild(b) }
func (s *BuildStore) Upsert(b build.Build) error                   { return s.Store.UpsertBuild(b) }

type PodStore struct{ *Store }

func (s *PodStore) List() ([]pod.Launch, error) { return s.Store.ListPods() }
func (s *PodStore) Create(p pod.Launch) error   { return s.Store.CreatePod(p) }

type WorkspaceStore struct{ *Store }

func (s *WorkspaceStore) List() ([]workspace.Workspace, error)                    { return s.Store.ListWorkspaces() }
func (s *WorkspaceStore) GetByID(id string) (workspace.Workspace, bool, error)    { return s.Store.GetWorkspaceByID(id) }
func (s *WorkspaceStore) Create(w workspace.Workspace) error                       { return s.Store.CreateWorkspace(w) }
func (s *WorkspaceStore) Delete(id string) error                                   { return s.Store.DeleteWorkspace(id) }

type SessionStore struct{ *Store }

func (s *SessionStore) Create(item session.Session) error                 { return s.Store.CreateSession(item) }
func (s *SessionStore) Get(token string) (session.Session, bool, error)   { return s.Store.GetSession(token) }
func (s *SessionStore) Delete(token string) error                          { return s.Store.DeleteSession(token) }

type SecretStore struct{ *Store }

func (s *SecretStore) ListByUser(userID string) ([]secret.Secret, error)                { return s.Store.ListByUser(userID) }
func (s *SecretStore) GetByName(userID, name string) (secret.Secret, bool, error)       { return s.Store.GetByName(userID, name) }
func (s *SecretStore) Upsert(item secret.Secret) error                                   { return s.Store.Upsert(item) }
func (s *SecretStore) Delete(userID, name string) error                                  { return s.Store.Delete(userID, name) }

type DatasetStore struct{ *Store }

func (s *DatasetStore) ListByUser(userID string) ([]dataset.Dataset, error)              { return s.Store.ListDatasetsByUser(userID) }
func (s *DatasetStore) GetByID(id string) (dataset.Dataset, bool, error)                 { return s.Store.GetDatasetByID(id) }
func (s *DatasetStore) Create(item dataset.Dataset) error                                { return s.Store.CreateDataset(item) }
func (s *DatasetStore) Delete(id string) error                                            { return s.Store.DeleteDataset(id) }

type RepositoryStore struct{ *Store }

func (s *RepositoryStore) ListByUser(userID string) ([]repository.Repository, error)     { return s.Store.ListRepositoriesByUser(userID) }
func (s *RepositoryStore) GetByID(id string) (repository.Repository, bool, error)        { return s.Store.GetRepositoryByID(id) }
func (s *RepositoryStore) Create(item repository.Repository) error                        { return s.Store.CreateRepository(item) }

type ProjectResourceStore struct{ *Store }
