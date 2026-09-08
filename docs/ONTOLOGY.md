# Ontology

**Where this is today, in one line:** it reads a dataset's object *paths* -
never their content - and infers a first draft of the objects in it: a study,
its pseudonymised subjects, their visits, modalities, formats and volumes.

The name stays. "Ontology" is the category the product is aiming at and the
word a buyer recognises, and renaming the feature to match what it does today
would trade the ambition for accuracy about a version that will not last.

What had to go were the specific promises around it. The empty screen said the
ontology was there "to query the platform in natural language"; the query
filters a manifest by subject, visit and modality, so the screen now says
filter and the field offers `ANTERION` rather than "how many orders were
placed last month". A word that names a direction is fair; a sentence that
describes a capability nobody built is not.

**What makes the word true**, and it is open: declared objects with properties
and links, bound to data, with this scan as their first step rather than their
competitor - the inference proposes Study, Subject, Visit and Modality, and a
person confirms, renames and links them.

The test of whether it is worth building was that it had to turn a selection
into a mount, and that part now exists. Three properties hold it up.

**Freshness.** An ontology is a photograph and the screen presented it as a
fact: the June scan of PREMYOM1000 read "18,738 objects, 20 subjects" in the
same typeface as September's 24,179 and 31, with nothing in between to say the
study had recruited eleven subjects. Opening an ontology now counts what the
source holds today and states the difference in objects. The count is a
listing: the platform reads how many objects are there and writes nothing back.

**Coverage.** "31 subjects" averaged together the subjects who carry a corneal
wavefront and those who do not. Coverage names them - how many subjects hold
each modality, sparsest first, and the identifiers of those who do not - so a
cohort's owner learns what it excludes before publishing an n rather than after.

**Cohorts.** A cohort is a named selection, resolved to an explicit list of
files at the moment it is declared and kept that way: a cohort that re-ran its
filter would return a different study every month. It duplicates nothing. The
frozen paths point into the dataset where the data already lives, and a
workspace mounts the cohort as a tree of symlinks at
`/mnt/cohorts/<name>/<subject>/<visit>/<modality>` over the read-only dataset
mount. The file list travels in the workspace bootstrap secret, which caps a
cohort at roughly 700 KB of compressed paths; past that the bootstrap log says
the cohort was not mounted rather than building a partial tree that would
silently be a different study.

Storing the paths is what made cohorts possible - the manifest keeps three
sample paths per modality, which shows a person what the data looks like and
could never define a study. Ontologies scanned before this refuse to produce a
cohort and ask for a rescan, rather than returning an empty selection that
looks like a legitimate answer.

Throughout, the source bucket is read-only to the platform: it is listed, and
never written, copied or modified.


NoryxLab can generate a first semantic catalog from datasets attached to the active project. The UI entry point belongs to the Data domain, not to the project resource panel, while the stored manifest remains project-scoped for RBAC and dataset attachment checks.

The current dataset inference is intentionally profile-based, not generic. The first supported profile is `noryx-file-path-v1`, built for the Noryx/FOR file layout.

## Scope

The `noryx-file-path-v1` profile scans S3 object metadata only and tries to infer:

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

Ontologies are first-class Data objects, like datasets:

- owner user or organization
- `owner`, `writer`, `reader` permissions
- source dataset or datasource
- inference profile
- manifest JSON
- project attachments through `project_ontology_links`

The legacy project-scoped manifest endpoint is kept for UI compatibility and “latest scan” display, but the source of truth for sharing and project attachment is the ontology object.

The stored manifest contains:

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
