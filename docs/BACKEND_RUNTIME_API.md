# Backend Runtime API (CE)

This module adds:

- docker image build submission (Kaniko `Job`)
- pod launch submission (`Pod`)
- workspace launch submission (Jupyter `Pod` + `Service`)
- project-scoped RBAC checks at API level
- OIDC authentication with Keycloak bearer tokens
- Swagger UI + OpenAPI spec
- split namespace runtime (`noryx-ce` control-plane, `noryx-loads` workloads)

## Endpoints

- `GET /swagger`
- `GET /swagger/openapi.yaml`
- `POST /api/v1/projects`
- `PUT /api/v1/projects/{projectID}/members/{userID}/role`
- `POST /api/v1/builds`
- `POST /api/v1/pods`
- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `DELETE /api/v1/workspaces/{workspaceID}`
- `POST /api/v1/auth/session`
- `DELETE /api/v1/auth/session`
- `/workspaces/{workspaceID}/...` (reverse-proxied Jupyter)
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/modules`

## Auth

Mutating and admin routes require:

- `Authorization: Bearer <access_token>`

Compatibility fallback:

- If no bearer token is provided, API still accepts `X-Noryx-User` header (temporary bootstrap mode).

## RBAC model

- project creator is set to `admin`
- `admin` can assign `viewer|editor|admin`
- `editor` and `admin` can submit builds and launch pods
- `editor` and `admin` can launch/delete/access workspaces

## Workspace baseline (current)

- kind: `jupyter`
- image: `harbor.lan/noryx-ce/noryx-workspace-jupyter:0.1.0`
- resources: requests=limits=`500m` CPU, `512Mi` memory
- ingress path: `/workspaces/{workspaceID}/...` routed to `noryx-back`
- web access auth: Keycloak bearer exchanged for secure HTTP-only session cookie
- workloads are created in `NORYX_WORKLOAD_NAMESPACE` (current deployment: `noryx-loads`)
- global admin is granted by realm role `noryx-admin`
- bootstrap global admin can be forced with `NORYX_BOOTSTRAP_ADMIN_USER`

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
    "destinationImage":"harbor.lan/noryx-ce/hello-world:test1"
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
