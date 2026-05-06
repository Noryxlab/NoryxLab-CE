package postgres

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
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
)

type ProjectStore struct{ *Store }
type AccessStore struct{ *Store }

func (s *ProjectStore) List() ([]project.Project, error) { return s.Store.List() }
func (s *ProjectStore) Create(p project.Project) error   { return s.Store.Create(p) }
func (s *ProjectStore) DeleteProject(id string) error    { return s.Store.DeleteProject(id) }

type BuildStore struct{ *Store }

func (s *BuildStore) List() ([]build.Build, error)                 { return s.Store.ListBuilds() }
func (s *BuildStore) GetByID(id string) (build.Build, bool, error) { return s.Store.GetBuildByID(id) }
func (s *BuildStore) Create(b build.Build) error                   { return s.Store.CreateBuild(b) }
func (s *BuildStore) Upsert(b build.Build) error                   { return s.Store.UpsertBuild(b) }

type AppStore struct{ *Store }

func (s *AppStore) List() ([]app.App, error)                 { return s.Store.ListApps() }
func (s *AppStore) GetByID(id string) (app.App, bool, error) { return s.Store.GetAppByID(id) }
func (s *AppStore) GetBySlug(slug string) (app.App, bool, error) {
	return s.Store.GetAppBySlug(slug)
}
func (s *AppStore) Create(item app.App) error { return s.Store.CreateApp(item) }
func (s *AppStore) Upsert(item app.App) error { return s.Store.UpsertApp(item) }
func (s *AppStore) Delete(id string) error    { return s.Store.DeleteApp(id) }

type JobStore struct{ *Store }

func (s *JobStore) List() ([]job.Job, error)                 { return s.Store.ListJobs() }
func (s *JobStore) GetByID(id string) (job.Job, bool, error) { return s.Store.GetJobByID(id) }
func (s *JobStore) Create(item job.Job) error                { return s.Store.CreateJob(item) }
func (s *JobStore) Upsert(item job.Job) error                { return s.Store.UpsertJob(item) }
func (s *JobStore) Delete(id string) error                   { return s.Store.DeleteJob(id) }

type PodStore struct{ *Store }

func (s *PodStore) List() ([]pod.Launch, error) { return s.Store.ListPods() }
func (s *PodStore) Create(p pod.Launch) error   { return s.Store.CreatePod(p) }

type WorkspaceStore struct{ *Store }

func (s *WorkspaceStore) List() ([]workspace.Workspace, error) { return s.Store.ListWorkspaces() }
func (s *WorkspaceStore) GetByID(id string) (workspace.Workspace, bool, error) {
	return s.Store.GetWorkspaceByID(id)
}
func (s *WorkspaceStore) Create(w workspace.Workspace) error { return s.Store.CreateWorkspace(w) }
func (s *WorkspaceStore) Delete(id string) error             { return s.Store.DeleteWorkspace(id) }

type SessionStore struct{ *Store }

func (s *SessionStore) Create(item session.Session) error { return s.Store.CreateSession(item) }
func (s *SessionStore) Get(token string) (session.Session, bool, error) {
	return s.Store.GetSession(token)
}
func (s *SessionStore) Delete(token string) error { return s.Store.DeleteSession(token) }

type SecretStore struct{ *Store }

func (s *SecretStore) ListByUser(userID string) ([]secret.Secret, error) {
	return s.Store.ListByUser(userID)
}
func (s *SecretStore) GetByName(userID, name string) (secret.Secret, bool, error) {
	return s.Store.GetByName(userID, name)
}
func (s *SecretStore) Upsert(item secret.Secret) error  { return s.Store.Upsert(item) }
func (s *SecretStore) Delete(userID, name string) error { return s.Store.Delete(userID, name) }

type DatasetStore struct{ *Store }

func (s *DatasetStore) ListByUser(userID string) ([]dataset.Dataset, error) {
	return s.Store.ListDatasetsByUser(userID)
}
func (s *DatasetStore) GetByID(id string) (dataset.Dataset, bool, error) {
	return s.Store.GetDatasetByID(id)
}
func (s *DatasetStore) Create(item dataset.Dataset) error { return s.Store.CreateDataset(item) }
func (s *DatasetStore) Delete(id string) error            { return s.Store.DeleteDataset(id) }

type DatasourceStore struct{ *Store }

func (s *DatasourceStore) ListByUser(userID string) ([]datasource.Datasource, error) {
	return s.Store.ListDatasourcesByUser(userID)
}
func (s *DatasourceStore) GetByID(id string) (datasource.Datasource, bool, error) {
	return s.Store.GetDatasourceByID(id)
}
func (s *DatasourceStore) Create(item datasource.Datasource) error { return s.Store.CreateDatasource(item) }
func (s *DatasourceStore) Delete(id string) error                  { return s.Store.DeleteDatasource(id) }

type RepositoryStore struct{ *Store }

func (s *RepositoryStore) ListByUser(userID string) ([]repository.Repository, error) {
	return s.Store.ListRepositoriesByUser(userID)
}
func (s *RepositoryStore) GetByID(id string) (repository.Repository, bool, error) {
	return s.Store.GetRepositoryByID(id)
}
func (s *RepositoryStore) Create(item repository.Repository) error {
	return s.Store.CreateRepository(item)
}
func (s *RepositoryStore) Delete(id string) error { return s.Store.DeleteRepository(id) }

type ProjectResourceStore struct{ *Store }
type UserPreferenceStore struct{ *Store }

func (s *ProjectResourceStore) AttachDataset(projectID, datasetID string) error {
	return s.Store.AttachDataset(projectID, datasetID)
}
func (s *ProjectResourceStore) DetachDataset(projectID, datasetID string) error {
	return s.Store.DetachDataset(projectID, datasetID)
}
func (s *ProjectResourceStore) ListProjectDatasetIDs(projectID string) ([]string, error) {
	return s.Store.ListProjectDatasetIDs(projectID)
}
func (s *ProjectResourceStore) AttachRepository(projectID, repositoryID string) error {
	return s.Store.AttachRepository(projectID, repositoryID)
}
func (s *ProjectResourceStore) DetachRepository(projectID, repositoryID string) error {
	return s.Store.DetachRepository(projectID, repositoryID)
}
func (s *ProjectResourceStore) ListProjectRepositoryIDs(projectID string) ([]string, error) {
	return s.Store.ListProjectRepositoryIDs(projectID)
}
func (s *ProjectResourceStore) AttachDatasource(projectID, datasourceID string) error {
	return s.Store.AttachDatasource(projectID, datasourceID)
}
func (s *ProjectResourceStore) DetachDatasource(projectID, datasourceID string) error {
	return s.Store.DetachDatasource(projectID, datasourceID)
}
func (s *ProjectResourceStore) ListProjectDatasourceIDs(projectID string) ([]string, error) {
	return s.Store.ListProjectDatasourceIDs(projectID)
}

func (s *UserPreferenceStore) Get(userID, key string) (string, bool, error) {
	return s.Store.GetUserPreference(userID, key)
}
func (s *UserPreferenceStore) Set(userID, key, value string) error {
	return s.Store.SetUserPreference(userID, key, value)
}
