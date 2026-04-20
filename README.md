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

- `GET /healthz`
- `GET /api/v1/projects`
- `POST /api/v1/projects`

## Kubernetes bootstrap

```bash
kubectl apply -k deploy/k8s/base
```

See `docs/BOOTSTRAP_VM.md` for VM preparation.
