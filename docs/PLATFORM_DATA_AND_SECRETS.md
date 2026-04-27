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
- `PUT /api/v1/datasets/{datasetID}/objects/{path...}`

Storage:

- object upload target is MinIO (`NORYX_MINIO_*`)
- bucket is created automatically at dataset creation if missing

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
