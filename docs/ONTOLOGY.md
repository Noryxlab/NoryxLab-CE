# Semantic Catalog / Ontology MVP

NoryxLab can generate a first semantic catalog from datasets attached to the active project. The UI entry point belongs to the Data domain, not to the project resource panel, while the stored manifest remains project-scoped for RBAC and dataset attachment checks.

## Scope

The MVP scans S3 object metadata only:

- object keys / paths
- object sizes
- file extensions and inferred formats
- subject identifiers inferred from paths
- visit dates inferred from paths
- modality names inferred from paths
- CSV/TSV table names inferred from filenames

For HDS datasets, the scan does not download object content. It does not parse DICOM tags, CSV rows, PDF content, images, or Excel values. Datasource catalogs currently use connection metadata only; SQL/NoSQL schema introspection is a later explicit step.

## UI

The catalog is exposed from `Data > Catalogue sémantique`. Users select a source dataset or datasource accessible from the Data domain, then create a catalog. This keeps the product model Palantir-like at the data layer while preserving project-level access control. Generated catalogs are also project resources: they can be attached to or detached from projects through the project resource panel, with the same operating model as datasets.

## API

```http
GET /api/v1/ontologies
GET /api/v1/projects/{projectID}/ontology
POST /api/v1/projects/{projectID}/ontology/scans
GET /api/v1/projects/{projectID}/ontologies
PUT /api/v1/projects/{projectID}/ontologies/{ontologyID}
DELETE /api/v1/projects/{projectID}/ontologies/{ontologyID}
```

Scan payload:

```json
{
  "sourceType": "dataset",
  "datasetId": "dataset-id"
}
```

Datasource payload:

```json
{
  "sourceType": "datasource",
  "datasourceId": "datasource-id"
}
```


## Output Model

The stored manifest is scoped to a project and contains:

- project and dataset identifiers
- study name inferred from subject identifiers
- global summary: subjects, visits, modalities, objects, size, formats, measurement tables
- subjects
- visits per subject
- modalities per visit
- sample object paths per modality

## HDS Safety

The ontology MVP intentionally avoids exposing health metadata extracted from file contents. Pseudonymized IDs, visit dates and object paths are still health-context metadata and must be handled as HDS data in Enterprise Edition.

Future healthcare services may add controlled PHI checks, integrity checks and deeper schema extraction, but they must remain explicit and audited.
