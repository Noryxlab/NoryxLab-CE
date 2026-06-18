# Semantic Catalog / Ontology MVP

NoryxLab can generate a first project-level semantic catalog from datasets attached to a project.

## Scope

The MVP scans S3 object metadata only:

- object keys / paths
- object sizes
- file extensions and inferred formats
- subject identifiers inferred from paths
- visit dates inferred from paths
- modality names inferred from paths
- CSV/TSV table names inferred from filenames

For HDS datasets, the scan does not download object content. It does not parse DICOM tags, CSV rows, PDF content, images, or Excel values.

## API

```http
GET /api/v1/projects/{projectID}/ontology
POST /api/v1/projects/{projectID}/ontology/scans
```

Scan payload:

```json
{
  "datasetId": "dataset-id-attached-to-project"
}
```

If `datasetId` is omitted, the backend scans the first dataset attached to the project.

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
