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

## A workspace is disposable

It is the one thing to know before using one. A workspace is a machine you are
lent, not a machine you own: it has a maximum lifetime and the platform stops
it when that is reached, its node can be replaced, and neither event is
supposed to cost you more than the time to relaunch.

So the rule for anybody working in one is short: **code goes to a repository,
data goes to object storage.** The project volume is the scratch space in
between - convenient, shared by the project, and not the place anything should
only exist.

The backup policy follows from this rather than the other way round: the
nightly volume backup covers volumes a workspace is using and skips those whose
last workspace has stopped (see [Recovery](RECOVERY.md)). That is deliberate,
and it is only safe because of the sentence above. Work that exists solely
inside a stopped workspace is not backed up by the platform and is not meant to
be.

## Git identity

At startup, a workspace configures a fallback Git author identity from the
authenticated Keycloak user:

- `user.name` uses the Keycloak username.
- `user.email` uses the Keycloak email, with a local fallback when unavailable.
- The Git configuration is stored in the persistent user profile and reused by
  subsequent workspaces.
- Each attached repository can override the fallback with its explicit
  `gitAuthorName` and `gitAuthorEmail`. These values are configured locally in
  that repository after clone.

Repository authentication remains managed by the attached repository
credential and its `PersAT` or `PrAT` type. Git author identity and repository
authentication are separate.

Repository safeguards:

- an explicit repository author identity must provide both name and email;
- repository connectivity is validated when the repository is created,
  updated, manually re-tested, and attached to a project;
- the latest validation result and timestamp are visible in the repository
  catalog;
- an expired or missing Git secret blocks repository attachment and workload
  launch with an explicit error;
- workload Git authentication is non-interactive and tokens are never embedded
  in repository URLs.

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

Default workspace base images:

- `harbor.example.local/noryx-environments/noryx-jupyter:0.1.0`
- `harbor.example.local/noryx-environments/noryx-vscode:0.1.0`
- `harbor.example.local/noryx-environments/noryx-rstudio:0.1.0`

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

- the cluster requires the GeeseFS-based S3 CSI driver, installed with `scripts/ops/install-s3-csi.sh`
- workspace PVC mount path is `/mnt` (project persistent)
- project PVC is created as `project-<projectId>` and reused across workspaces
- the project page exposes this PVC as a secured file explorer, without requiring an active workspace
- project viewers can browse and download files; editors and owners can upload, edit and delete files
- `/mnt/requirements.txt` is auto-applied at workspace startup (project venv: `/mnt/.venv`)
- IDE tooling auto-update runs in background once/day by default:
  - VS Code extensions (Python/Jupyter/Data Science baseline)
  - Jupyter/Python packages (`ipywidgets`, `jupyterlab-git`, `jupyterlab-lsp`, `python-lsp-server`, `black`, `isort`, `ruff`, ...)
  - disable with `NORYX_AUTO_UPDATE_IDE=0`
  - logs: `/tmp/noryx-ide-tooling.log`, `/tmp/noryx-vscode-extensions.log`, `/tmp/noryx-jupyter-extensions.log`
- `/repos` is ephemeral (workspace-local, non-persistent)
- `/datasets` is reserved for datasets mounts
- attached datasets are resolved through dataset RBAC, including organization ownership and ACLs
- every attached S3 bucket is mounted directly with the cluster S3 CSI driver under `/datasets/<dataset-name>`
- dataset contents are never pre-copied into the workspace ephemeral filesystem or project PVC
- datasets granted through the `reader` role are mounted read-only
- Enterprise workspaces can mount attached HDS datasets when the HDS feature is enabled
- `/home/noryx/.noryx-profile` is mounted from user-scoped profile PVC (RWX)
- active user secrets are exposed through Kubernetes `secretKeyRef` variables named
  `NORYX_SECRET_<NORMALIZED_NAME>`; dataset credentials and expired secrets are excluded
- runtime user is `noryx` with `sudo` enabled in both system workspace images
- workload pods have no mounted Kubernetes ServiceAccount token and use the
  network isolation baseline from `docs/WORKLOAD_NETWORK_ISOLATION.md`
- workloads use named resource profiles documented in `docs/HARDWARE_TIERS.md`

Reference:

- `docs/WORKSPACE_FILESYSTEM_LAYOUT.md`
- `docs/S3_DATASET_MOUNTS.md`

## Build base images with Noryx

Dockerfile paths in this repo:

- `environments/noryx-jupyter/Dockerfile`
- `environments/noryx-vscode/Dockerfile`
- `environments/noryx-rstudio/Dockerfile`

Use `POST /api/v1/builds` with:

- a Dockerfile inheriting from the matching system image
- a project-specific `destinationImage`

The workspace launcher selects both an IDE and a compatible environment image.
Jobs, applications and dashboards select an environment image without launching an IDE.
Custom workspace images must inherit from the matching system image so the Noryx
runtime user, paths and IDE binary remain available.

## Notes

- project membership is enforced (`editor` or `admin` required)
- `GET /api/v1/workspaces` returns only workspaces from projects where caller has a role
- workspace access always requires a Keycloak identity (`Authorization: Bearer ...` or `noryx_session`) and project RBAC authorization
- sharing a workspace URL does not grant access to another user
- workspace URL returned by API:
  - Jupyter: `/workspaces/<workspaceID>/lab?reset`
  - VSCode: `/workspaces/<workspaceID>/?folder=/mnt`
- wildcard DNS is not required: workspace traffic stays on `https://datalab.example.local/workspaces/<workspaceID>/...`
- `harbor-regcred` must exist in workload namespace for image pull
- Longhorn must be installed and healthy for workspace creation when PVC is enabled
- metadata stores are currently in-memory (restart resets records)

## Workspace Open Flow (UI)

Current front behavior (`ce-web-0.6.17+`):

1. user clicks `Open` in Workspaces tab
2. front opens `about:blank` in a new tab
3. front calls `POST /api/v1/auth/session` to ensure `noryx_session`
4. tab is redirected to `/workspaces/<workspaceID>/lab?reset`
5. backend validates the authenticated user and project RBAC on every proxied request
6. backend injects the internal Jupyter token only on the private proxy-to-workspace request

This avoids home-page replacement and reduces browser-specific blank-page issues.

## Troubleshooting

For a dedicated runbook, see:

- `docs/WORKSPACE_TROUBLESHOOTING.md`

## The assistant reads workspace files, including regulated ones

The developer assistant stays available in a workspace that mounts a dataset
classified `hds`, and the platform says so on the launch sheet before the
workspace is created, naming the datasets concerned.

**The exposure, stated plainly.** The assistant's tools read the filesystem as
the person who opened the workspace, and whatever they read is sent to the
model to answer with. On 2026-09-18 an assistant was asked to list `/datasets`
and returned the contents of an HDS dataset - file names describing a study,
its modalities and its cohorts - which then left the site for the model
endpoint, through a gateway, to a rented GPU. That was not a misuse: it is what
the tool does, and the platform's own rules text invites it to use `/datasets`.

**Why it is not blocked.** Withholding the credential closes it completely, and
was implemented on 2026-09-20 before being reverted the same day. It trades the
wrong thing. Working on regulated data with current tooling is the reason this
platform exists for its first customer, and an assistant that switches itself
off exactly where the work happens removes the product in order to protect it.
The decision is Stephane's, taken with the measurement above in hand.

So the control here is disclosure rather than prevention, and the honest thing
is to name that rather than describe it as a safeguard. Every launch of a
workspace that mounts regulated data with the assistant enabled is logged, so
the question "how often did this happen" has an answer rather than an estimate.

**What actually closes it.** Serving the model inside the site, where nothing
it is told has to leave. The hardware is already at EMSE. That makes the
question disappear instead of trading it, and it is the only version of this
an auditor can be shown - a rule in the prompt is not a control, since the
model may ignore it and the tools never read it.

Until then, two things follow. Anyone deploying this platform for regulated
data should know that the assistant's model endpoint is part of the data
boundary, and choosing a hosted model puts the data there. And a reader of a
compliance report should not find this paragraph surprising, which is why it is
written here rather than discovered later.
