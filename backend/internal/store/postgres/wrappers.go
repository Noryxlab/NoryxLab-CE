package postgres

import (
	"encoding/json"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/egress"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/pod"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type ProjectStore struct{ *Store }
type AccessStore struct{ *Store }

func (s *ProjectStore) List() ([]project.Project, error) { return s.Store.List() }
func (s *ProjectStore) Create(p project.Project) error   { return s.Store.Create(p) }
func (s *ProjectStore) UpdateMetadata(projectID, name, description string) error {
	return s.Store.UpdateProjectMetadata(projectID, name, description)
}
func (s *ProjectStore) UpdateOwner(projectID, ownerType, ownerID string) error {
	return s.Store.UpdateProjectOwner(projectID, ownerType, ownerID)
}
func (s *ProjectStore) DeleteProject(id string) error { return s.Store.DeleteProject(id) }

func (s *AccessStore) SetRole(projectID, userID string, role access.Role) {
	s.Store.SetRole(projectID, userID, role)
}
func (s *AccessStore) GetRole(projectID, userID string) (access.Role, bool) {
	return s.Store.GetRole(projectID, userID)
}
func (s *AccessStore) ListProjectRoles() ([]store.ProjectRole, error) {
	return s.Store.ListProjectRoles()
}

type BuildStore struct{ *Store }

func (s *BuildStore) List() ([]build.Build, error)                 { return s.Store.ListBuilds() }
func (s *BuildStore) GetByID(id string) (build.Build, bool, error) { return s.Store.GetBuildByID(id) }
func (s *BuildStore) Create(b build.Build) error                   { return s.Store.CreateBuild(b) }
func (s *BuildStore) Upsert(b build.Build) error                   { return s.Store.UpsertBuild(b) }
func (s *BuildStore) Delete(id string) error                       { return s.Store.DeleteBuild(id) }

type AppStore struct{ *Store }

func (s *AppStore) List() ([]app.App, error)                 { return s.Store.ListApps() }
func (s *AppStore) GetByID(id string) (app.App, bool, error) { return s.Store.GetAppByID(id) }
func (s *AppStore) GetBySlug(slug string) (app.App, bool, error) {
	return s.Store.GetAppBySlug(slug)
}
func (s *AppStore) Create(item app.App) error { return s.Store.CreateApp(item) }
func (s *AppStore) Upsert(item app.App) error { return s.Store.UpsertApp(item) }
func (s *AppStore) Delete(id string) error    { return s.Store.DeleteApp(id) }
func (s *AppStore) ListRevisions(appID string) ([]app.Revision, error) {
	return s.Store.ListAppRevisions(appID)
}
func (s *AppStore) CreateRevision(item app.Revision) error {
	return s.Store.CreateAppRevision(item)
}
func (s *AppStore) ActivateRevision(appID, revisionID string) error {
	return s.Store.ActivateAppRevision(appID, revisionID)
}

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

type AuditStore struct{ *Store }
type EgressRuleStore struct{ *Store }

func (s *AuditStore) Create(event audit.Event) error { return s.Store.CreateAuditEvent(event) }
func (s *AuditStore) List(filter store.AuditFilter) ([]audit.Event, error) {
	return s.Store.ListAuditEvents(filter)
}

func (s *EgressRuleStore) List() ([]egress.Rule, error) {
	return s.Store.ListEgressRules()
}
func (s *EgressRuleStore) ListByProject(projectID string) ([]egress.Rule, error) {
	return s.Store.ListEgressRulesByProject(projectID)
}
func (s *EgressRuleStore) GetByID(id string) (egress.Rule, bool, error) {
	return s.Store.GetEgressRuleByID(id)
}
func (s *EgressRuleStore) Create(item egress.Rule) error {
	return s.Store.CreateEgressRule(item)
}
func (s *EgressRuleStore) UpdateDecision(id, status, reviewerID, note string) error {
	return s.Store.UpdateEgressRuleDecision(id, status, reviewerID, note)
}

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

func (s *DatasetStore) ListBySubjects(subjects []dataset.Subject) ([]dataset.Dataset, error) {
	return s.Store.ListDatasetsBySubjects(subjects)
}
func (s *DatasetStore) ListAll() ([]dataset.Dataset, error) { return s.Store.ListAllDatasets() }
func (s *DatasetStore) GetByID(id string) (dataset.Dataset, bool, error) {
	return s.Store.GetDatasetByID(id)
}
func (s *DatasetStore) Create(item dataset.Dataset) error { return s.Store.CreateDataset(item) }
func (s *DatasetStore) UpdateMetadata(datasetID, name, description string) error {
	return s.Store.UpdateDatasetMetadata(datasetID, name, description)
}
func (s *DatasetStore) Delete(id string) error { return s.Store.DeleteDataset(id) }
func (s *DatasetStore) ListAccess(datasetID string) ([]dataset.Access, error) {
	return s.Store.ListDatasetAccess(datasetID)
}
func (s *DatasetStore) UpdateOwner(datasetID, ownerType, ownerID string) error {
	return s.Store.UpdateDatasetOwner(datasetID, ownerType, ownerID)
}
func (s *DatasetStore) GetAccess(datasetID, subjectType, subjectID string) (dataset.Access, bool, error) {
	return s.Store.GetDatasetAccess(datasetID, subjectType, subjectID)
}
func (s *DatasetStore) SetAccess(item dataset.Access) error { return s.Store.SetDatasetAccess(item) }
func (s *DatasetStore) DeleteAccess(datasetID, subjectType, subjectID string) error {
	return s.Store.DeleteDatasetAccess(datasetID, subjectType, subjectID)
}

type OntologyStore struct{ *Store }

func (s *OntologyStore) ListBySubjects(subjects []ontology.Subject) ([]ontology.Ontology, error) {
	return s.Store.ListOntologiesBySubjects(subjects)
}
func (s *OntologyStore) ListAll() ([]ontology.Ontology, error) { return s.Store.ListAllOntologies() }
func (s *OntologyStore) GetByID(id string) (ontology.Ontology, bool, error) {
	return s.Store.GetOntologyByID(id)
}
func (s *OntologyStore) Create(item ontology.Ontology) error { return s.Store.CreateOntology(item) }
func (s *OntologyStore) UpdateMetadata(ontologyID, name, description string) error {
	return s.Store.UpdateOntologyMetadata(ontologyID, name, description)
}
func (s *OntologyStore) Delete(id string) error { return s.Store.DeleteOntology(id) }
func (s *OntologyStore) ListAccess(ontologyID string) ([]ontology.Access, error) {
	return s.Store.ListOntologyAccess(ontologyID)
}
func (s *OntologyStore) UpdateOwner(ontologyID, ownerType, ownerID string) error {
	return s.Store.UpdateOntologyOwner(ontologyID, ownerType, ownerID)
}
func (s *OntologyStore) GetAccess(ontologyID, subjectType, subjectID string) (ontology.Access, bool, error) {
	return s.Store.GetOntologyAccess(ontologyID, subjectType, subjectID)
}
func (s *OntologyStore) SetAccess(item ontology.Access) error {
	return s.Store.SetOntologyAccess(item)
}
func (s *OntologyStore) DeleteAccess(ontologyID, subjectType, subjectID string) error {
	return s.Store.DeleteOntologyAccess(ontologyID, subjectType, subjectID)
}

type DatasourceStore struct{ *Store }

func (s *DatasourceStore) ListByUser(userID string) ([]datasource.Datasource, error) {
	return s.Store.ListDatasourcesByUser(userID)
}
func (s *DatasourceStore) ListAll() ([]datasource.Datasource, error) {
	return s.Store.ListAllDatasources()
}
func (s *DatasourceStore) GetByID(id string) (datasource.Datasource, bool, error) {
	return s.Store.GetDatasourceByID(id)
}
func (s *DatasourceStore) Create(item datasource.Datasource) error {
	return s.Store.CreateDatasource(item)
}
func (s *DatasourceStore) Upsert(item datasource.Datasource) error {
	return s.Store.UpsertDatasource(item)
}
func (s *DatasourceStore) Delete(id string) error { return s.Store.DeleteDatasource(id) }

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
func (s *RepositoryStore) Update(item repository.Repository) error {
	return s.Store.UpdateRepository(item)
}
func (s *RepositoryStore) Delete(id string) error { return s.Store.DeleteRepository(id) }

type ProjectResourceStore struct{ *Store }
type ProjectOntologyStore struct{ *Store }
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
func (s *ProjectResourceStore) ListDatasourceProjectIDs(datasourceID string) ([]string, error) {
	return s.Store.ListDatasourceProjectIDs(datasourceID)
}
func (s *ProjectResourceStore) AttachOntology(projectID, ontologyID string) error {
	return s.Store.AttachOntology(projectID, ontologyID)
}
func (s *ProjectResourceStore) DetachOntology(projectID, ontologyID string) error {
	return s.Store.DetachOntology(projectID, ontologyID)
}
func (s *ProjectResourceStore) ListProjectOntologyIDs(projectID string) ([]string, error) {
	return s.Store.ListProjectOntologyIDs(projectID)
}

func (s *ProjectOntologyStore) GetProjectOntology(projectID string) (json.RawMessage, bool, error) {
	return s.Store.GetProjectOntology(projectID)
}
func (s *ProjectOntologyStore) UpsertProjectOntology(projectID, datasetID string, manifest json.RawMessage, generatedBy string) error {
	return s.Store.UpsertProjectOntology(projectID, datasetID, manifest, generatedBy)
}

func (s *UserPreferenceStore) Get(userID, key string) (string, bool, error) {
	return s.Store.GetUserPreference(userID, key)
}
func (s *UserPreferenceStore) Set(userID, key, value string) error {
	return s.Store.SetUserPreference(userID, key, value)
}
