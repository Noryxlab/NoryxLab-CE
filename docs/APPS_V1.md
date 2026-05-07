# Noryx CE Apps V1

Apps V1 allows project members to deploy long-running web services and expose them with a user-defined URL slug.

## Access model

- App traffic is exposed through backend proxy routes:
  - `/apps/{slug}`
  - `/apps/{slug}/{path...}`
- Access requires authenticated platform identity (Keycloak session cookie or Bearer token).
- Project RBAC check is enforced on each proxied request (`CanLaunchPod` capability).

## Runtime model

- Namespace: `noryx-loads`
- Pod label: `app.kubernetes.io/name=noryx-app`
- Default port: `9000` (distinct from workspace `8888`)
- Service per app (ClusterIP)
- URL is path-based by slug (no wildcard DNS required)

## API (V1)

- `GET /api/v1/apps?projectId=<id>`
- `POST /api/v1/apps`
- `DELETE /api/v1/apps/{appID}`

Create payload example:

```json
{
  "projectId": "PROJECT_ID",
  "name": "Fraud UI",
  "slug": "fraud-ui",
  "image": "harbor.lan/noryx-environments/noryx-python:0.2.3",
  "port": 9000,
  "args": ["python3 -m http.server 9000 --bind 0.0.0.0 --directory /mnt"]
}
```

Notes:

- If `args` is empty, backend starts a default static server command.
- Slug must match: lowercase `[a-z0-9-]` and stay unique cluster-wide in V1.

## App entrypoint resolution

App startup follows this order:

1. UI command (`command/args`) when provided
2. `/mnt/app.sh` when present
3. fallback static HTTP server on selected port

Standard recommended format is `/mnt/app.sh` (single entrypoint style).

## Bootstrap behavior

At app startup:

- project mounts are prepared (`/mnt`, `/repos`, `/datasets`)
- repos attached to the project are cloned/pulled
- datasets attached to the project are synced (initial sync)
- if `/mnt/requirements.txt` exists, dependencies are installed (venv + user)

Bootstrap logs include:

- `[bootstrap] requirements detected at /mnt/requirements.txt`
- `[bootstrap] requirements installation completed`
- or no-file message when absent.
