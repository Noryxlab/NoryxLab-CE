# Workspaces (Jupyter V1)

Current CE baseline for workspaces:

- kind: `jupyter`
- runtime: one Kubernetes pod per workspace
- service: one ClusterIP service per workspace (port `8888`)
- resources: request=limit `500m` CPU, `512Mi` memory

## API

- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`

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
- metadata stores are currently in-memory (restart resets records)
