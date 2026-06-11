# Platform Data, Secrets, and Repository Catalog (CE baseline)

This baseline introduces three user-scoped catalogs persisted in PostgreSQL:

- secrets (`/api/v1/secrets`)
- datasets (`/api/v1/datasets`)
- repositories (`/api/v1/repositories`)

Project-scoped attachments are also available:

- datasets attached to a project
- repositories attached to a project

## Persistence backend

Noryx backend now defaults to PostgreSQL store mode:

- `NORYX_STORE_BACKEND=postgres`
- `NORYX_DATABASE_HOST=postgres`
- `NORYX_DATABASE_NAME=noryx`
- `NORYX_DATABASE_USER=noryx`
- `NORYX_DATABASE_PASSWORD` from `noryx-secrets`

Runtime records are persisted and survive backend pod restart:

- projects
- project roles
- builds
- pods
- workspaces
- web sessions
- user secrets
- datasets
- repositories
- project dataset/repository attachments

## Secrets

Endpoints:

- `GET /api/v1/secrets`
- `POST /api/v1/secrets`
- `DELETE /api/v1/secrets/{name}`

Notes:

- values are encrypted before DB storage (AES-GCM)
- encryption key from `NORYX_SECRETS_MASTER_KEY`
- list endpoint returns metadata only (no clear value)

## Datasets

Endpoints:

- `GET /api/v1/datasets`
- `POST /api/v1/datasets`
- `GET /api/v1/datasets/{datasetID}/objects`
- `GET /api/v1/datasets/{datasetID}/objects/{path...}`
- `PUT /api/v1/datasets/{datasetID}/objects/{path...}`
- `POST /api/v1/datasets/{datasetID}/download`
- `POST /api/v1/datasets/{datasetID}/download-url`
- `GET /api/v1/datasets/{datasetID}/access`
- `PUT /api/v1/datasets/{datasetID}/access/{userID}`
- `DELETE /api/v1/datasets/{datasetID}/access/{userID}`

Storage:

- object upload target is MinIO (`NORYX_MINIO_*`)
- bucket is created automatically at dataset creation if missing
- object listing returns files below the dataset prefix
- CE manages standard datasets (`classification=non-hds`); regulated HDS datasets require Enterprise Edition
- Clever Cloud Cellar datasets are registered with `provider=clever-cloud`, an existing bucket, and an endpoint
- datasets can be attached to or detached from projects by their owner or a global admin
- external S3 credentials are provided at dataset creation, encrypted with the platform master key, and never exposed in dataset records or secret APIs
- each external dataset uses its own dedicated credentials; there is no shared-profile fallback
- external endpoints must use HTTPS
- dataset permissions are persisted per user:
  - `owner`: paternity, read/write, delete, and permission management
  - `writer`: read and object upload/update
  - `reader`: read only
- only the owner or a global admin can manage permissions
- dataset owners and ACL subjects can be users or Keycloak organizations
- organization members receive the dataset permissions granted to their organization
- ownership can be transferred between a user and an organization; technical S3 credentials remain attached to their original encrypted secret
- browser preview supports PDF, images, CSV, and text formats
- browser editing supports CSV and text formats for owners and writers
- the dataset explorer exposes S3 prefixes as familiar folders; owners and writers can create folders and delete files or folders recursively
- attached S3 datasets are mounted directly into workloads; see `docs/S3_DATASET_MOUNTS.md`
- XLSX/ODS files share one SheetJS-based browser viewer/editor for simple tabular values; saving can alter complex formulas, styles, macros, charts, and layouts

External dataset creation:

- provide endpoint, region, existing bucket, optional prefix, access key, and secret key
- the backend validates bucket access before encrypting credentials and creating the dataset
- use distinct service accounts and least-privilege IAM policies for external buckets

## Repositories

Endpoints:

- `GET /api/v1/repositories`
- `POST /api/v1/repositories`
- `PUT /api/v1/repositories/{repositoryID}`

Repository records separate:

- author identity (`gitAuthorName`, `gitAuthorEmail`) used for commits
- authentication type (`persat`, `prat`, or `none`)
- the user secret reference (`authSecretName`) used for clone, pull, and push

Tokens remain stored as encrypted secrets and are never returned in repository records.
Repository connectivity status is persisted after creation, update, manual
validation, and project attachment. Attachment is rejected when the repository
or its current credential is no longer accessible. Expired Git secrets are
rejected before workload launch.

## Project attachments

Dataset attachment:

- `GET /api/v1/projects/{projectID}/datasets`
- `PUT /api/v1/projects/{projectID}/datasets/{datasetID}`
- `DELETE /api/v1/projects/{projectID}/datasets/{datasetID}`

Repository attachment:

- `GET /api/v1/projects/{projectID}/repositories`
- `PUT /api/v1/projects/{projectID}/repositories/{repositoryID}`
- `DELETE /api/v1/projects/{projectID}/repositories/{repositoryID}`

RBAC:

- listing attached resources: project member required
- attach/detach: editor/admin required
