package store

type ProjectResourceStore interface {
	AttachDataset(projectID, datasetID string) error
	DetachDataset(projectID, datasetID string) error
	ListProjectDatasetIDs(projectID string) ([]string, error)
	// The other direction, for every attachable object.
	//
	// Attaching only ever had one direction on screen: a project listed what
	// it mounted, and an object said nothing about where it was mounted. For
	// an extract that is the wrong way round - it exists to be reused by
	// several projects, which is the whole point of a link rather than an
	// appartenance - and somebody looking at one had no way to ask the
	// question from there.
	ListDatasetProjectIDs(datasetID string) ([]string, error)
	AttachRepository(projectID, repositoryID string) error
	DetachRepository(projectID, repositoryID string) error
	ListProjectRepositoryIDs(projectID string) ([]string, error)
	AttachDatasource(projectID, datasourceID string) error
	DetachDatasource(projectID, datasourceID string) error
	ListProjectDatasourceIDs(projectID string) ([]string, error)
	ListDatasourceProjectIDs(datasourceID string) ([]string, error)
	AttachOntology(projectID, ontologyID string) error
	DetachOntology(projectID, ontologyID string) error
	ListProjectOntologyIDs(projectID string) ([]string, error)
	// An extract attaches like everything else above it.
	//
	// It used to carry a project_id instead, decided when it was declared and
	// never afterwards - the same precondition the ontology shed, one layer
	// down. An extract describes a selection over a dataset; which projects
	// mount it is a separate question, asked later and answerable more than
	// once.
	AttachExtract(projectID, extractID string) error
	DetachExtract(projectID, extractID string) error
	ListProjectExtractIDs(projectID string) ([]string, error)
	ListExtractProjectIDs(extractID string) ([]string, error)
	// The other direction: an extract declared from the catalogue has to know
	// which project will mount it, and an ontology that belongs to exactly one
	// project can answer that itself.
	ListOntologyProjectIDs(ontologyID string) ([]string, error)
}
