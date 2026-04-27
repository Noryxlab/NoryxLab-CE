package store

type ProjectResourceStore interface {
	AttachDataset(projectID, datasetID string) error
	DetachDataset(projectID, datasetID string) error
	ListProjectDatasetIDs(projectID string) ([]string, error)
	AttachRepository(projectID, repositoryID string) error
	DetachRepository(projectID, repositoryID string) error
	ListProjectRepositoryIDs(projectID string) ([]string, error)
}
