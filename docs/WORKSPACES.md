# Workspaces (Jupyter V1)

Current CE baseline for workspaces:

- kind: `jupyter`
- runtime: one Kubernetes pod per workspace
- service: one ClusterIP service per workspace (port `8888`)
- resources: request=limit `500m` CPU, `512Mi` memory
- workload namespace: `noryx-loads` (via `NORYX_WORKLOAD_NAMESPACE`)

## API

- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `DELETE /api/v1/workspaces/{workspaceID}`
- `POST /api/v1/auth/session` (create browser session from bearer)
- `DELETE /api/v1/auth/session`
- `/workspaces/{workspaceID}/...` (Jupyter reverse proxy)

Create payload:

```json
{
  "projectId": "<project-id>",
  "name": "jupyter-demo"
}
```

## Image used

Default workspace image:

- `harbor.lan/noryx-ce/noryx-workspace-jupyter:0.1.0`

Configurable with env var:

- `NORYX_WORKSPACE_JUPYTER_IMAGE`

## Build the base image with Noryx

Dockerfile path in this repo:

- `environments/jupyter-debian/Dockerfile`

Use `POST /api/v1/builds` with:

- `dockerfilePath`: `environments/jupyter-debian/Dockerfile`
- `destinationImage`: `harbor.lan/noryx-ce/noryx-workspace-jupyter:0.1.0`

## Notes

- project membership is enforced (`editor` or `admin` required)
- `GET /api/v1/workspaces` returns only workspaces from projects where caller has a role
- Jupyter access path is guarded by Keycloak identity (bearer or web session cookie)
- workspace URL returned by API points to `/workspaces/<workspaceID>/lab`
- `harbor-regcred` must exist in workload namespace for image pull
- metadata stores are currently in-memory (restart resets records)
