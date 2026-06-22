# Semantic Catalog / Ontology MVP

NoryxLab can generate a first semantic catalog from datasets attached to the active project. The UI entry point belongs to the Data domain, not to the project resource panel, while the stored manifest remains project-scoped for RBAC and dataset attachment checks.

The current dataset inference is intentionally profile-based, not generic. The first supported profile is `premyom-file-path-v1`, built for the Premyom/FOR file layout.

## Scope

The `premyom-file-path-v1` profile scans S3 object metadata only and tries to infer:

- a study, for example `PREMYOM1000`
- pseudonymized subjects/patients, for example `PREMYOM1000-0001`
- visits/dates when present in paths
- modalities, for example `ANTERION`, `IOLMASTER`
- formats, for example `CSV`, `PDF`, `XLSX`
- measurement tables from CSV/TSV filenames, for example `Cornea_Basics`
- object counts and byte sizes

For HDS datasets, the scan does not download object content. It does not parse DICOM tags, CSV rows, PDF content, images, or Excel values. On another dataset layout, this profile can produce incomplete or irrelevant output; adding a new domain requires adding a new explicit inference profile.

Datasource catalogs currently use the `datasource-metadata-v1` profile: connection metadata only, without SQL/NoSQL schema introspection. Real DB schema extraction is a later explicit step.

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
- inference profile
- study name inferred from subject identifiers
- global summary: subjects, visits, modalities, objects, size, formats, measurement tables
- subjects
- visits per subject
- modalities per visit
- sample object paths per modality

## HDS Safety

The ontology MVP intentionally avoids exposing health metadata extracted from file contents. Pseudonymized IDs, visit dates and object paths are still health-context metadata and must be handled as HDS data in Enterprise Edition.

Future healthcare services may add controlled PHI checks, integrity checks and deeper schema extraction, but they must remain explicit and audited.
