# Backend Runtime API (CE)

This module adds:

- docker image build submission (Kaniko `Job`)
- environment catalog from build revisions
- pod launch submission (`Pod`)
- workspace launch submission (Jupyter `Pod` + `Service`)
- workspace volume lifecycle (PVC per workspace)
- project-scoped RBAC checks at API level
- OIDC authentication with Keycloak bearer tokens
- Swagger UI + OpenAPI spec
- split namespace runtime (`noryx-ce` control-plane, `noryx-loads` workloads)
- PostgreSQL persistence for platform objects (projects, RBAC, builds, workspaces, sessions, catalogs)

## Endpoints

- `GET /api/v1/platform/overview`: aggregate home-page activity metrics; storage volume only includes CE datasets whose S3 buckets can be measured by the backend

- `GET /swagger`
- `GET /swagger/openapi.yaml`
- `GET /api/v1/user/preferences`
- `PUT /api/v1/user/preferences`
- `POST /api/v1/projects`
- `PUT /api/v1/projects/{projectID}/members/{userID}/role`
- `GET /api/v1/builds`
- `POST /api/v1/builds`
- `GET /api/v1/builds/{buildID}/dockerfile`
- `GET /api/v1/environments`
- `POST /api/v1/pods`
- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `DELETE /api/v1/workspaces/{workspaceID}`
- `GET /api/v1/secrets`
- `POST /api/v1/secrets`
- `DELETE /api/v1/secrets/{name}`
- `GET /api/v1/datasets`
- `POST /api/v1/datasets`
- `GET /api/v1/datasets/{datasetID}/objects`
- `GET /api/v1/datasets/{datasetID}/objects/{path...}`
- `PUT /api/v1/datasets/{datasetID}/objects/{path...}`
- `POST /api/v1/datasets/{datasetID}/download`
- `POST /api/v1/datasets/{datasetID}/download-url`
- `GET /api/v1/datasets/{datasetID}/access`
- `PUT /api/v1/datasets/{datasetID}/ownership`
- `PUT|DELETE /api/v1/datasets/{datasetID}/access/{subjectType}/{subjectID}`
- `PUT /api/v1/datasets/{datasetID}/access/{userID}`
- `DELETE /api/v1/datasets/{datasetID}/access/{userID}`
- `GET /api/v1/repositories`
- `POST /api/v1/repositories`
- `GET /api/v1/projects/{projectID}/datasets`
- `PUT /api/v1/projects/{projectID}/datasets/{datasetID}`
- `DELETE /api/v1/projects/{projectID}/datasets/{datasetID}`
- `GET /api/v1/projects/{projectID}/repositories`
- `PUT /api/v1/projects/{projectID}/repositories/{repositoryID}`
- `DELETE /api/v1/projects/{projectID}/repositories/{repositoryID}`
- `POST /api/v1/auth/session`
- `DELETE /api/v1/auth/session`
- `/workspaces/{workspaceID}/...` (reverse-proxied Jupyter)
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/modules`
- `GET /api/v1/admin/audit`
- `GET|POST /api/v1/admin/organizations`
- `DELETE /api/v1/admin/organizations/{organizationID}`
- `GET /api/v1/admin/organizations/{organizationID}/members`
- `PUT|DELETE /api/v1/admin/organizations/{organizationID}/members/{userID}`

User preferences:

- `language` (`fr` or `en`) is persisted server-side per user
- `theme` (`noryx`) is persisted server-side per user; branded themes are supplied by Enterprise Edition overlays
- frontend uses browser language by default, then applies stored preference when present
- frontend theme falls back to admin default from backend (`NORYX_UI_DEFAULT_THEME`) when user preference is not set
- the response includes the current user's Keycloak organizations so the UI can display identity context

## Auth

Mutating and admin routes require:

- `Authorization: Bearer <access_token>`

When `NORYX_ORGANIZATION_REQUIRED=true`, every authenticated request also
requires at least one Keycloak organization membership. See
`docs/ORGANIZATIONS.md`.

Compatibility fallback:

- If no bearer token is provided, API still accepts `X-Noryx-User` header (temporary bootstrap mode).

Workspace reverse proxy auth (`/workspaces/{workspaceID}/...`):

- bearer or `noryx_session` is mandatory
- project RBAC (`editor|admin`) is mandatory
- workspace URLs never expose the internal Jupyter token
- backend injects the internal token only when proxying an authorized request to the workspace

## RBAC model

- project creator is set to `admin`
- `admin` can assign `viewer|editor|admin`
- `POST /api/v1/projects/{projectID}/invitations` invites one user with role (`editor` by default)
- `editor` and `admin` can submit builds and launch pods
- `editor` and `admin` can launch/delete/access workspaces
- every successful or failed `POST`, `PUT`, `PATCH`, and `DELETE` request under
  `/api/v1` is audited without request payloads; detailed business events are
  additionally emitted for sensitive operations
- audit retention: no purge policy is applied by default in CE (append-only audit table)
- first authenticated `GET /api/v1/projects` auto-provisions a default project for users without project membership
- CE bootstrap admin (`NORYX_BOOTSTRAP_ADMIN_USER`) has cross-project visibility and bypasses project membership checks
- datasets can be attached to or detached from projects by their owner or the global admin
- Clever Cloud datasets are registered by an admin; external object access requires platform-injected service credentials
- external S3 credentials are encrypted per dataset with no shared credential fallback
- dataset ACL roles are `owner`, `writer`, and `reader`
- dataset owners and ACL subjects may be `user` or `organization`
- owner manages dataset ACLs
- external datasets use temporary S3 download URLs for large single files; datasets also support multi-file ZIP download, preview, and text/CSV editing
- XLSX and ODS use the same browser spreadsheet viewer/editor; editing is limited to simple cell values and may alter advanced workbook features
- regulated HDS dataset management is an Enterprise Edition extension and is rejected by CE

## Workspace baseline (current)

- kind: `jupyter` or `vscode`
- Jupyter base image: `harbor.lan/noryx-environments/noryx-jupyter:0.1.0`
- VSCode base image: `harbor.lan/noryx-environments/noryx-vscode:0.1.0`
- RStudio base image: `harbor.lan/noryx-environments/noryx-rstudio:0.1.0`
- runtime pod naming: `wks-<shortid>`
- resources: requests=limits=`500m` CPU, `512Mi` memory
- project volume: shared `PersistentVolumeClaim` per project (`longhorn`, `ReadWriteMany`, `10Gi`, mount `/mnt`)
- project file API: `GET|PUT|DELETE /api/v1/projects/{projectID}/files/{path}` and `POST /api/v1/projects/{projectID}/folders`
- user profile volume: `PersistentVolumeClaim` per user (`longhorn`, `ReadWriteMany`, default `5Gi`, mount `/home/noryx/.noryx-profile`)
- `POST /api/v1/workspaces` accepts optional `storageSize` to override default PVC size per workspace
- ingress path: `/workspaces/{workspaceID}/...` routed to `noryx-backend`
- web access auth:
  - Keycloak bearer exchanged for secure HTTP-only session cookie (`noryx_session`)
  - project RBAC checked on every workspace proxy request
  - internal Jupyter token never exposed in public workspace URLs
- workloads are created in `NORYX_WORKLOAD_NAMESPACE` (current deployment: `noryx-loads`)
- global admin for CE operations is controlled by `NORYX_BOOTSTRAP_ADMIN_USER`
- workspace bootstrap:
  - `/mnt/requirements.txt` is auto-applied at startup (project virtualenv `/mnt/.venv`)
  - `/repos` is workspace-local and non-persistent
  - `/datasets` is reserved for dataset mounts

## Quick test

```bash
BASE="https://datalab.noryxlab.ai"

PROJECT_ID=$(curl -sk -X POST "$BASE/api/v1/projects" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: alice' \
  -d '{"name":"demo"}' | jq -r '.id')

curl -sk -X PUT "$BASE/api/v1/projects/$PROJECT_ID/members/bob/role" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: alice' \
  -d '{"role":"editor"}'

curl -sk -X POST "$BASE/api/v1/builds" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: bob' \
  -d '{
    "projectId":"'"$PROJECT_ID"'",
    "gitRepository":"https://github.com/docker-library/hello-world.git",
    "gitRef":"master",
    "dockerfilePath":"Dockerfile",
    "destinationImage":"harbor.lan/noryx-environments/hello-world:test1"
  }'

curl -sk -X POST "$BASE/api/v1/pods" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: bob' \
  -d '{
    "projectId":"'"$PROJECT_ID"'",
    "image":"busybox:1.36",
    "command":["sh","-c"],
    "args":["echo noryx && sleep 5"]
  }'
```
