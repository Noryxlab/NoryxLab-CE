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
- `GET /api/v1/datasets/{datasetID}/access`
- `PUT /api/v1/datasets/{datasetID}/access/{userID}`
- `DELETE /api/v1/datasets/{datasetID}/access/{userID}`

Storage:

- object upload target is MinIO (`NORYX_MINIO_*`)
- bucket is created automatically at dataset creation if missing
- object listing returns files below the dataset prefix
- datasets are classified as `non-hds` or `hds`
- Clever Cloud Cellar datasets are registered with `provider=clever-cloud`, an existing bucket, and an endpoint
- HDS datasets can only be attached to or detached from projects by a global admin
- non-HDS datasets can be attached to or detached from projects by their owner or a global admin
- external S3 credentials are provided at dataset creation, encrypted with the platform master key, and never exposed in dataset records or secret APIs
- each external dataset uses its own dedicated credentials; there is no shared-profile fallback
- HDS S3 never falls back to another dataset credential, standard S3, or internal MinIO
- external and HDS endpoints must use HTTPS
- HDS credentials are not injected into user workspaces until a dedicated workload-identity or secret-injection flow is implemented
- dataset permissions are persisted per user:
  - `owner`: paternity, read/write, delete, and permission management
  - `writer`: read and object upload/update
  - `reader`: read only
- only the owner or a global admin can manage non-HDS permissions
- only a global admin can manage HDS permissions
- direct file download, multi-file ZIP download, preview, and browser editing are disabled for HDS datasets
- non-HDS browser preview supports PDF, images, CSV, and text formats
- non-HDS browser editing supports CSV and text formats for owners and writers
- XLSX/ODS files are download-only until a dedicated spreadsheet engine is integrated

External dataset creation:

- provide endpoint, region, existing bucket, optional prefix, access key, and secret key
- the backend validates bucket access before encrypting credentials and creating the dataset
- use distinct service accounts and IAM policies for every HDS bucket

## Repositories

Endpoints:

- `GET /api/v1/repositories`
- `POST /api/v1/repositories`

Repository records can reference a user secret by name (`authSecretName`) for later git auth flows.

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
