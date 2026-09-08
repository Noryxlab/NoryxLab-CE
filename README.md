# NoryxLab Community Edition

NoryxLab is a Kubernetes-native platform for collaborative data work. It gives
Python and R teams one project-oriented place to manage data, repositories,
environments, interactive workspaces, jobs and applications.

The Community Edition (CE) is the public, self-hosted foundation of NoryxLab.
It is designed to run on infrastructure you control, with S3-compatible object
storage and Keycloak-based authentication.

> **Status:** actively developed. The supported deployment and API contracts
> are documented below; upgrade testing on a representative non-production
> cluster is recommended before production use.

## What You Can Do

- Create collaborative projects and attach datasets, Git repositories,
  environments, secrets and data connections.
- Register S3-compatible datasets and mount them directly into workloads.
- Build and manage curated or custom container environments from Dockerfiles.
- Launch Jupyter, VS Code and RStudio workspaces with persistent project files.
- Run one-off and scheduled Kubernetes jobs.
- Publish project applications and operate their lifecycle through the
  Production view.
- Define named hardware tiers for workload limits.
- Authenticate users through Keycloak and manage project-level access.
- Consume a supported, versioned REST API and its local Swagger UI.

## Editions

CE contains the reusable platform core: projects, data, repositories,
environments, workloads, applications, S3 integration and baseline access
control.

NoryxLab Enterprise Edition is distributed separately and adds organisation
governance, advanced RBAC, audit, quotas, platform validation, controlled
egress, backup operations and other enterprise capabilities. The CE repository
does not contain Enterprise source code or a runtime switch that unlocks it.
See [the edition model](docs/EDITIONS.md) and
[the CE/EE extension boundary](docs/EE_EXTENSION_POINTS.md).

## Architecture

```text
Users
  |
  v
Noryx frontend  --->  Noryx API  ---> PostgreSQL
       |                  |
       |                  +-------> Keycloak (OIDC)
       |                  +-------> S3-compatible storage
       |                  +-------> Kubernetes API
       |
       +-- projects, datasets, environments, workspaces, jobs, apps

Kubernetes workloads
  |- workspaces (Jupyter, VS Code, RStudio)
  |- jobs and scheduled jobs
  `- published applications
```

The control plane runs in Kubernetes. Workloads run as Kubernetes resources in
a dedicated workload namespace. Harbor and the image build service are external
components in the reference deployment.

## Quick Start for Development

Prerequisites:

- Go `1.25`
- Node.js `22` and npm
- PostgreSQL `16` for database-backed backend tests

Run the API in local development mode:

```bash
git clone https://github.com/Noryxlab/NoryxLab-CE.git
cd NoryxLab-CE
go -C backend run ./cmd/noryx-api
```

The local API exposes `/healthz` and Swagger at `/swagger`. Local development
uses the in-memory store unless PostgreSQL configuration is supplied. Do not
enable header authentication outside a local development environment.

Run the frontend:

```bash
npm --prefix frontend ci
npm --prefix frontend run dev
```

Run the main quality checks before submitting a change:

```bash
go -C backend vet ./...
go -C backend test ./...
npm --prefix frontend run typecheck
npm --prefix frontend run lint
npm --prefix frontend run test
npm --prefix frontend run build
./scripts/check-edition-boundary.sh
```

## Deploy on Kubernetes

The reference topology requires:

- a Kubernetes control-plane VM or cluster;
- an external Harbor registry;
- an external Docker build service;
- S3-compatible storage, initially MinIO or an external endpoint;
- Keycloak for OpenID Connect authentication;
- a public DNS name and TLS termination.

Start with the infrastructure prerequisites and VM bootstrap documents:

1. [Infrastructure prerequisites](docs/INFRA_PREREQUISITES.md)
2. [VM bootstrap](docs/BOOTSTRAP_VM.md)
3. [Keycloak setup](docs/KEYCLOAK_SETUP.md)
4. [Image security and Harbor mirroring](docs/IMAGE_SECURITY.md)

The baseline manifests are in `deploy/k8s/base`:

```bash
kubectl apply --server-side --force-conflicts -k deploy/k8s/base
kubectl -n noryx-ce get pods
```

For a production platform, use the documented deployment automation rather
than treating this minimal command as a complete installation procedure.

## Security Model

- Production authentication is OIDC bearer-token validation against Keycloak.
- `X-Noryx-User` is only accepted with `NORYX_AUTH_MODE=header`, for local
  development; it is rejected under OIDC.
- Service-to-service calls use `X-Noryx-Service-Token`.
- Projects are the collaboration boundary. Project creators become admins;
  admins manage project roles.
- Datasets are accessed through configured S3 endpoints and mounted directly
  into workloads. Credentials are not embedded in images.

Read [the RBAC model](docs/RBAC_MODEL.md),
[Keycloak setup](docs/KEYCLOAK_SETUP.md) and
[workload network isolation](docs/WORKLOAD_NETWORK_ISOLATION.md) before
exposing a platform to users.

## API

Noryx serves its API documentation from the running platform:

- `/swagger`: supported public API
- `/swagger/openapi.public.yaml`: supported OpenAPI contract
- `/swagger?spec=full`: full implementation inventory, including internal UI
  routes without compatibility guarantees

The API is versioned under `/api/v1`. CI checks that its OpenAPI documents stay
in sync with the router. See [API documentation](docs/API.md).

## Documentation

### Platform concepts

- [Projects](docs/PROJECTS.md)
- [Datasets and S3 mounts](docs/S3_DATASET_MOUNTS.md)
- [Environments](docs/ENVIRONMENTS.md)
- [Workspaces](docs/WORKSPACES.md)
- [Jobs and schedules](docs/JOB_HISTORY.md) and [scheduled jobs](docs/SCHEDULED_JOBS.md)
- [Applications and Production](docs/APPS_V1.md) and [Production](docs/PRODUCTION.md)
- [Data sources](docs/DATASOURCES_V1.md)
- [Hardware tiers](docs/HARDWARE_TIERS.md)

### Operations

- [Backend runtime API](docs/BACKEND_RUNTIME_API.md)
- [Workspace troubleshooting](docs/WORKSPACE_TROUBLESHOOTING.md)
- [Recovery](docs/RECOVERY.md)
- [Observability](docs/OBSERVABILITY.md)
- [Platform data and secrets](docs/PLATFORM_DATA_AND_SECRETS.md)

## Contributing

Contributions are welcome. Before opening a pull request:

1. Keep CE source independent from Enterprise source code.
2. Add or update tests for behavioural changes.
3. Run the checks listed in the development section.
4. Update the supported API document when changing a public route:

```bash
python3 scripts/ops/generate-openapi.py
```

5. Describe the user-facing impact and deployment implications in the pull
   request.

For security-sensitive issues, do not publish credentials, customer data or
exploit details in a public issue. Contact the maintainers privately first.

## License

NoryxLab Community Edition is licensed under the
[Mozilla Public License 2.0](LICENSE) (`MPL-2.0`). The MPL applies at file
level: modifications to MPL-covered files remain under MPL-2.0 when
distributed, while separate files can be combined with the Community Edition as
part of a larger work under different terms.
