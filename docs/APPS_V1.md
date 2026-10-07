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
- `GET /api/v1/apps/{appID}/usage`
- `POST /api/v1/apps/{appID}/publish`
- `GET /api/v1/apps/{appID}/revisions`
- `POST /api/v1/apps/{appID}/revisions/{revisionID}/rollback`
- `GET /api/v1/production/apps`
- `POST /api/v1/apps/{appID}/restart`
- `POST /api/v1/apps/{appID}/stop`
- `DELETE /api/v1/apps/{appID}`

Lifecycle semantics:

- `restart` recreates an active pod from its current Kubernetes specification;
- `stop` removes the pod but keeps the application record and URL reservation;
- `delete` permanently removes the record, pod, service and workload secrets;
- a stopped application must be deployed again rather than restarted from a
  potentially stale runtime specification.

## Usage analytics

Enterprise audit records one `app.view` event when the root document of an app
is opened. Requests for assets, API paths and WebSockets are not counted as
views.

The usage endpoint aggregates the latest 30 days:

- total document views;
- identified visitors and their last visit;
- anonymous views for public apps;
- daily view counts.

Public anonymous visitors are intentionally not fingerprinted. Their IP address
remains restricted to the Enterprise audit log and is not exposed by the app
usage endpoint.

## Production revisions

Creating or running an app does not publish it. Publication is explicit and
creates an immutable numbered revision containing:

- the Noryx application configuration;
- the complete sanitized Kubernetes pod manifest;
- publisher and publication timestamp.

Rollback restores the selected pod manifest and marks the selected revision as
active. Existing applications remain development-only until first publication.

Create payload example:

```json
{
  "projectId": "PROJECT_ID",
  "name": "Fraud UI",
  "slug": "fraud-ui",
  "image": "harbor.example.local/my-project/my-app:1.0.0",
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

### The dependency check

After the install and before the launch, when the launch command names a Python
file (`streamlit run app.py`, `python3 main.py`), the bootstrap reads that file's
imports and compares them with what is installed and what `requirements.txt`
declares. It writes one of three things:

```
[deps] MANQUANT  app.py importe 'plotly' et rien ne le fournit.
[deps]           L'application va echouer. Ajoutez 'plotly' a /mnt/requirements.txt.
```

The application is going to crash on its first request. The module and the
package to add are named, which beats the traceback the browser shows.

```
[deps] FRAGILE   app.py importe 'plotly', fourni par 'plotly',
[deps]           que /mnt/requirements.txt ne declare pas. Ca marche maintenant
[deps]           et cassera au prochain redemarrage du conteneur,
[deps]           qui ne rejoue que /mnt/requirements.txt.
```

This is the one that matters. The module is importable — somebody ran `pip
install` in the running container — so the application works today. **A pod is
not a disk.** At the next restart the platform replays `requirements.txt` and
nothing else, and the package is gone. The restart may be months away and will
look like an unrelated platform fault. This warning was available at every start
of the application that motivated it, for a month, before it broke.

```
[deps] les imports de app.py sont tous declares.
```

What it does not do, on purpose:

- **It never fails the launch.** An application that half works is worth more
  to its owner than a refusal, and a check that can take an application down is
  a check somebody will have removed.
- It reads only the entry file, not every script in the project — warning about
  a stray script teaches the owner to ignore warnings.
- It skips the standard library, relative imports, and anything inside a string
  (imports are parsed with `ast`, not matched by regexp).
- With no Python entry file (a shell entrypoint, the static server) there is no
  check and no line: a reassuring message about a file never opened would be
  worse than silence.

A handful of packages whose import name differs from their package name are
mapped (`cv2` → `opencv-python`, `sklearn` → `scikit-learn`, `yaml` →
`PyYAML`, `PIL` → `pillow`). For anything else the module name is suggested,
which is right far more often than not.
