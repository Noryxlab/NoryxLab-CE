# NoryxLab-CE

Noryx Community Edition codebase.

## Preamble: required infrastructure

Before cluster bootstrap, prepare:

- one Harbor VM (registry)
- one dockerbuild VM (build/push)

Control-plane images expected in Harbor project `noryx-ce`:

- `noryx-backend:<tag>`
- `noryx-frontend:<tag>`

Details: `docs/INFRA_PREREQUISITES.md`.

## V1 bootstrap scope

- `backend/`: Noryx API skeleton (Go)
- `deploy/k8s/base/`: baseline manifests for:
  - Noryx Front
  - PostgreSQL
  - Keycloak
  - MinIO
  - Noryx Back API

## Quick start (local dev)

```bash
cd backend
go run ./cmd/noryx-api
```

API endpoints:

- `GET /` (minimal front with Keycloak login + admin API test buttons)
- `GET /healthz`
- `GET /swagger`
- `GET /swagger/openapi.yaml`
- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `PUT /api/v1/projects/{projectID}/members/{userID}/role`
- `GET /api/v1/builds`
- `POST /api/v1/builds`
- `GET /api/v1/builds/{buildID}/dockerfile`
- `GET /api/v1/environments`
- `GET /api/v1/pods`
- `POST /api/v1/pods`
- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `DELETE /api/v1/workspaces/{workspaceID}`
- `POST /api/v1/auth/session`
- `DELETE /api/v1/auth/session`
- `/workspaces/{workspaceID}/...` (Jupyter reverse proxy via back)
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/modules`

Auth:

- preferred: `Authorization: Bearer <access_token>` (Keycloak OIDC)
- temporary fallback: `X-Noryx-User` header (bootstrap mode)

Project RBAC:

- project creator gets `admin`
- `admin` can manage member roles
- `editor` and `admin` can submit builds and launch pods
- `editor` and `admin` can launch workspaces (Jupyter)
- first authenticated `GET /api/v1/projects` auto-creates a default project if user has none

## Kubernetes bootstrap

```bash
kubectl apply -k deploy/k8s/base
```

Runtime mode in cluster:

- API deployment enables in-cluster runtime (`NORYX_ENABLE_K8S_RUNTIME=true`)
- control-plane namespace: `NORYX_KUBE_NAMESPACE` (default `noryx-ce`)
- workload namespace: `NORYX_WORKLOAD_NAMESPACE` (default = control namespace; current manifest sets `noryx-loads`)
- API service account can create/delete `pods`, `services`, and `jobs`
- API service account can create/delete `persistentvolumeclaims` for workspace volumes
- registry credentials are read from secret name `harbor-regcred`
- OIDC issuer (current deployment): `https://datalab.noryxlab.ai/auth/realms/noryx`
- OIDC JWKS (current deployment): `http://keycloak:8080/auth/realms/noryx/protocol/openid-connect/certs`
- Keycloak base URL for admin API: `http://keycloak:8080/auth`
- ingress policy: `web` (80) redirects to HTTPS and `websecure` (443) is mandatory for user traffic
- security headers: HSTS enabled on `datalab.noryxlab.ai` (`max-age=31536000`, `includeSubDomains`, `preload`)
- DNS policy: wildcard records are not required. Noryx CE routes everything through one hostname (`datalab.noryxlab.ai`) with path-based routing (`/auth`, `/api`, `/swagger`, `/workspaces`).
- Longhorn CSI is installed by Ansible bootstrap and used for workspace PVCs (`NORYX_WORKSPACE_PVC_STORAGE_CLASS=longhorn`).

Important for split namespaces:

- create `harbor-regcred` in `noryx-loads` too (same credentials as `noryx-ce`)
- otherwise workspace pods and Kaniko jobs created in `noryx-loads` will fail on image pull/push

Keycloak bootstrap helper:

```bash
scripts/keycloak/bootstrap-realm.sh
```

Reference: `docs/BACKEND_RUNTIME_API.md`, `docs/KEYCLOAK_SETUP.md`.
Workspace module: `docs/WORKSPACES.md`.
Environment module: `docs/ENVIRONMENTS.md`.
Workspace runbook: `docs/WORKSPACE_TROUBLESHOOTING.md`.

See `docs/BOOTSTRAP_VM.md` for VM preparation.

## Ops helper

Cleanup all workspaces for a user (requires delete endpoint deployed):

```bash
BASE_URL=https://datalab.noryxlab.ai NORYX_USER=stef ./scripts/ops/cleanup-workspaces.sh
```
