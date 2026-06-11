# Noryx CE Apps V1

Apps V1 allows project members to deploy long-running web services and expose them with a user-defined URL slug.

## Access model

- App traffic is exposed through backend proxy routes:
  - `/apps/{slug}`
  - `/apps/{slug}/{path...}`
- Deployment and deletion require project RBAC `CanLaunchPod`.
- Visitor access is configured per app:
  - `private`: creator only
  - `users`: explicitly allowed Keycloak users
  - `organization`: members of explicitly allowed Keycloak organizations
  - `public`: no authentication
- Global administrators retain access to non-public apps.
- Existing apps created before app RBAC migration retain legacy project-member access.

## Runtime model

- Namespace: `noryx-loads`
- Pod label: `app.kubernetes.io/name=noryx-app`
- Default port: `9000` (distinct from workspace `8888`)
- Service per app (ClusterIP)
- URL is path-based by slug (no wildcard DNS required)

## API (V1)

- `GET /api/v1/apps?projectId=<id>`
- `POST /api/v1/apps`
- `GET /api/v1/apps/{appID}/logs?tailLines=500`
- `POST /api/v1/apps/{appID}/restart`
- `POST /api/v1/apps/{appID}/stop`
- `DELETE /api/v1/apps/{appID}`

Lifecycle semantics:

- `restart` recreates an active pod from its current Kubernetes specification;
- `stop` removes the pod but keeps the application record and URL reservation;
- `delete` permanently removes the record, pod, service and workload secrets;
- a stopped application must be deployed again rather than restarted from a
  potentially stale runtime specification.

Create payload example:

```json
{
  "projectId": "PROJECT_ID",
  "name": "Fraud UI",
  "slug": "fraud-ui",
  "image": "harbor.lan/my-project/my-app:1.0.0",
  "port": 9000,
  "args": []
}
```

Notes:

- If `args` is empty, the backend starts `/mnt/app.sh` when present, then falls
  back to a static server.
- Slug must match: lowercase `[a-z0-9-]` and stay unique cluster-wide in V1.

## App entrypoint resolution

App startup follows this order:

1. UI command (`command/args`) when provided
2. `/mnt/app.sh` when present
3. fallback static HTTP server on selected port

Standard recommended format is `/mnt/app.sh` (single entrypoint style). Keep
service preparation and launch logic in this script so workspace and app
execution use the same project-owned entrypoint.

The launcher exports `PORT` and `NORYX_APP_PORT` with the selected application
port before executing the script.

## Bootstrap behavior

At app startup:

- project mounts are prepared (`/mnt`, `/repos`, `/datasets`)
- repos attached to the project are cloned/pulled
- datasets attached to the project are mounted directly through S3 CSI under `/datasets`
- if `/mnt/requirements.txt` exists, dependencies are installed (venv + user)

Bootstrap logs include:

- `[bootstrap] requirements detected at /mnt/requirements.txt`
- `[bootstrap] requirements installation completed`
- or no-file message when absent.
