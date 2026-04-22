# NoryxLab-CE

Noryx Community Edition codebase.

## Preamble: required infrastructure

Before cluster bootstrap, prepare:

- one Harbor VM (registry)
- one dockerbuild VM (build/push)

Details: `docs/INFRA_PREREQUISITES.md`.

## V1 bootstrap scope

- `backend/`: Noryx API skeleton (Go)
- `deploy/k8s/base/`: baseline manifests for:
  - PostgreSQL
  - Keycloak
  - MinIO
  - Noryx API

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
- `GET /api/v1/pods`
- `POST /api/v1/pods`
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/modules`

Auth:

- preferred: `Authorization: Bearer <access_token>` (Keycloak OIDC)
- temporary fallback: `X-Noryx-User` header (bootstrap mode)

Project RBAC:

- project creator gets `admin`
- `admin` can manage member roles
- `editor` and `admin` can submit builds and launch pods

## Kubernetes bootstrap

```bash
kubectl apply -k deploy/k8s/base
```

Runtime mode in cluster:

- API deployment enables in-cluster runtime (`NORYX_ENABLE_K8S_RUNTIME=true`)
- API service account can create `pods` and `jobs`
- registry credentials are read from secret name `harbor-regcred`
- OIDC issuer (current deployment): `http://datalab.noryxlab.ai/auth/realms/noryx`
- OIDC JWKS (current deployment): `http://keycloak:8080/auth/realms/noryx/protocol/openid-connect/certs`
- Keycloak base URL for admin API: `http://keycloak:8080/auth`

Keycloak bootstrap helper:

```bash
scripts/keycloak/bootstrap-realm.sh
```

Reference: `docs/BACKEND_RUNTIME_API.md`, `docs/KEYCLOAK_SETUP.md`.

See `docs/BOOTSTRAP_VM.md` for VM preparation.
