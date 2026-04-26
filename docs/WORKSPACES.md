# Workspaces (Jupyter + VSCode V1)

Current CE baseline for workspaces:

- kind: `jupyter` or `vscode`
- runtime: one Kubernetes pod per workspace
- service: one ClusterIP service per workspace (port `8888`)
- project volume: one shared PVC per project (Longhorn by default)
- profile volume: one shared PVC per user for IDE settings (Longhorn RWX)
- resources: request=limit `500m` CPU, `512Mi` memory
- workload namespace: `noryx-loads` (via `NORYX_WORKLOAD_NAMESPACE`)
- generated runtime pod name prefix: `wks-`

## API

- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `DELETE /api/v1/workspaces/{workspaceID}`
- `POST /api/v1/auth/session` (create browser session from bearer)
- `DELETE /api/v1/auth/session`
- `/workspaces/{workspaceID}/...` (workspace reverse proxy)

Create payload:

```json
{
  "projectId": "<project-id>",
  "ide": "vscode",
  "name": "jupyter-demo",
  "storageSize": "20Gi"
}
```

`storageSize` is optional. If omitted, backend uses `NORYX_WORKSPACE_PVC_SIZE` (`10Gi` by default).

## Image used

Default workspace base image (shared by Jupyter + VSCode):

- `harbor.lan/noryx-environments/noryx-python:0.2.1`

Configurable with env var:

- `NORYX_WORKSPACE_JUPYTER_IMAGE`
- `NORYX_WORKSPACE_VSCODE_IMAGE`
- `NORYX_WORKSPACE_PVC_ENABLED` (`true` by default)
- `NORYX_WORKSPACE_PVC_STORAGE_CLASS` (`longhorn` by default)
- `NORYX_WORKSPACE_PVC_SIZE` (`10Gi` by default)
- `NORYX_WORKSPACE_PVC_ACCESS_MODE` (`ReadWriteMany` by default)
- `NORYX_WORKSPACE_PVC_MOUNT_PATH` (`/mnt` by default)
- `NORYX_WORKSPACE_PROFILE_PVC_ENABLED` (`true` by default)
- `NORYX_WORKSPACE_PROFILE_PVC_STORAGE_CLASS` (`longhorn` by default)
- `NORYX_WORKSPACE_PROFILE_PVC_SIZE` (`5Gi` by default)
- `NORYX_WORKSPACE_PROFILE_PVC_ACCESS_MODE` (`ReadWriteMany` by default)
- `NORYX_WORKSPACE_PROFILE_PVC_MOUNT_PATH` (`/home/noryx/.noryx-profile` by default)

## Filesystem layout

Current implementation baseline:

- workspace PVC mount path is `/mnt` (project persistent)
- project PVC is created as `project-<projectId>` and reused across workspaces
- `/mnt/requirements.txt` is auto-applied at workspace startup (project venv: `/mnt/.venv`)
- `/repos` is linked to project repository area under `/mnt`
- `/datasets` is reserved for datasets mounts
- `/home/noryx/.noryx-profile` is mounted from user-scoped profile PVC (RWX)
- runtime user is `noryx` with `sudo` enabled in `noryx-python` image

Reference:

- `docs/WORKSPACE_FILESYSTEM_LAYOUT.md`

## Build base images with Noryx

Dockerfile path in this repo:

- `environments/noryx-python/Dockerfile`

Use `POST /api/v1/builds` with:

- `dockerfilePath`: `environments/noryx-python/Dockerfile`
- `destinationImage`: `harbor.lan/noryx-environments/noryx-python:0.2.1`

## Notes

- project membership is enforced (`editor` or `admin` required)
- `GET /api/v1/workspaces` returns only workspaces from projects where caller has a role
- Jupyter access path supports two auth paths:
  - preferred: Keycloak identity (`Authorization: Bearer ...` or `noryx_session`) + project RBAC check
  - compatibility fallback: workspace URL token (`?token=...`) upgraded to a workspace-scoped cookie (`noryx_ws_token_<workspaceID>`)
- workspace URL returned by API:
  - Jupyter: `/workspaces/<workspaceID>/lab?reset`
  - VSCode: `/workspaces/<workspaceID>/?folder=/mnt`
- wildcard DNS is not required: workspace traffic stays on `https://datalab.noryxlab.ai/workspaces/<workspaceID>/...`
- `harbor-regcred` must exist in workload namespace for image pull
- Longhorn must be installed and healthy for workspace creation when PVC is enabled
- metadata stores are currently in-memory (restart resets records)

## Workspace Open Flow (UI)

Current front behavior (`ce-web-0.6.17+`):

1. user clicks `Open` in Workspaces tab
2. front opens `about:blank` in a new tab
3. front calls `POST /api/v1/auth/session` to ensure `noryx_session`
4. tab is redirected to `/workspaces/<workspaceID>/lab?reset&token=<workspace-token>`
5. backend sets workspace cookie `noryx_ws_token_<workspaceID>` (path: `/workspaces/<workspaceID>/`)
6. Jupyter static/API calls continue using that workspace cookie

This avoids home-page replacement and reduces browser-specific blank-page issues.

## Troubleshooting

For a dedicated runbook, see:

- `docs/WORKSPACE_TROUBLESHOOTING.md`
