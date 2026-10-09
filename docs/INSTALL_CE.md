# Install NoryxLab Community Edition

This guide takes a new Kubernetes installation to a usable Community Edition
baseline: OIDC login, projects, datasets, workspaces, jobs and applications.
It is intentionally explicit about the infrastructure an operator owns.

It does **not** install Enterprise capabilities such as the governance matrix,
advanced audit, controlled egress, backup operations or regulated-data
workflows. Those are not hidden in CE manifests and cannot be enabled with a
configuration switch.

## 0. Choose the installation boundary

Before running a command, decide and record these values. They become the
installation contract and belong in a private operations repository, not in a
fork of this public repository.

| Decision | Example | Why it matters |
| --- | --- | --- |
| Public URL | `https://datalab.example.org` | OIDC redirect URIs, ingress and TLS use the same hostname. |
| Edge TLS mode | Traefik or an external proxy | Only one component terminates TLS. |
| Registry | `harbor.example.org` | Every platform and workload image is pulled from it. |
| Storage class | `longhorn` | Project and user-profile volumes require ReadWriteMany support. |
| S3 endpoint | MinIO or external S3 | Datasets mount directly; they are not copied to workspace disks. |
| OIDC provider | Keycloak realm `noryx` | It is the source of users and sessions. |
| Workload namespace | `noryx-loads` | User workloads stay separate from the control plane. |

The base manifests use `noryx` for the control plane and `noryx-loads` for
workloads. Keep those names unless every manifest, secret and operational
command is changed together.

## 1. Pin the source version

Install from a tagged release or an explicit commit, never an unrecorded copy
of `main`.

```bash
git clone https://github.com/Noryxlab/NoryxLab-CE.git
cd NoryxLab-CE
git checkout <release-tag-or-approved-commit>
git rev-parse HEAD
```

Keep the resulting SHA with the installation configuration. It is the answer
to "which manifests produced this cluster?".

## 2. Verify the cluster before installing Noryx

Run these checks from the administration workstation. The account needs to
create namespaced resources, RBAC objects, CRDs already supplied by the
cluster ingress, and secrets in both Noryx namespaces.

```bash
kubectl get nodes
kubectl get storageclass
kubectl auth can-i create deployments --namespace noryx
kubectl auth can-i create deployments --namespace noryx-loads
kubectl auth can-i create secrets --namespace noryx
kubectl auth can-i create secrets --namespace noryx-loads
helm version
docker version
```

For persistent workspaces, verify that the selected storage class supports the
access mode configured for project and profile volumes. The default is
`ReadWriteMany`; a block-only storage class will leave workspaces pending.

Install the S3 CSI driver before the first dataset is mounted:

```bash
./scripts/ops/install-s3-csi.sh
kubectl get csidriver ru.yandex.s3.csi
```

See [infrastructure prerequisites](INFRA_PREREQUISITES.md) for the Harbor,
build host, DNS and CoreDNS requirements.

## 3. Prepare the registry and runtime images

The cluster must pull from a registry it can resolve and trust. Create these
projects in Harbor before deployment:

- `noryx-ce` for control-plane images;
- `noryx-environments` for Jupyter, VS Code and RStudio images;
- `noryx-dataservices` if internal data services will be offered.

Mirror the runtime images from a trusted build host. The catalog targets use
`harbor.example.local`; copy and edit the catalog in the private installation
repository before running the command.

The catalog pins the NoryxLab public mirror for the MinIO release used by CE.
This avoids relying on upstream anonymous registry access during a new
installation. It is an unchanged upstream MinIO image, not a Noryx image; its
origin, digest and maintenance policy are documented in
[Image mirrors](IMAGE_MIRRORS.md).

```bash
docker login "$HARBOR_HOST"
CATALOG_FILE=/path/to/installation/essential-images.txt \
  ./scripts/ops/sync-images-to-harbor.sh
```

Build and push the selected CE backend, frontend and system workspace images
under immutable tags. The images configured for `noryx-backend`,
`noryx-frontend`, `NORYX_PROJECT_FILES_IMAGE` and each workspace IDE must all
refer to the same approved release set.

Create the pull secret in both namespaces after they exist:

```bash
kubectl create namespace noryx --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace noryx-loads --dry-run=client -o yaml | kubectl apply -f -

for namespace in noryx noryx-loads; do
  kubectl -n "$namespace" create secret docker-registry harbor-regcred \
    --docker-server="$HARBOR_HOST" \
    --docker-username="$HARBOR_USERNAME" \
    --docker-password="$HARBOR_PASSWORD" \
    --dry-run=client -o yaml | kubectl apply -f -
done
```

For a production registry, mount its CA certificate as the optional
`harbor-ca` secret and set `NORYX_HARBOR_INSECURE_SKIP_VERIFY=false`. Do not
keep the repository's lab default of skipping registry TLS verification.

## 4. Create a private installation overlay

Do not edit and commit the public `deploy/k8s/base` directory. Create a private
Kustomize overlay or equivalent rendered-manifest pipeline which imports the
base and replaces installation-specific values. Store credentials in a secret
manager or an ignored environment file, never in Git.

At a minimum, replace every value in this table before the first apply:

| File in the CE base | Values that must be installation-specific |
| --- | --- |
| `secrets.yaml` | PostgreSQL, Keycloak and MinIO passwords; `NORYX_SECRETS_MASTER_KEY` |
| `noryx-api.yaml` | Registry hostname and image tags, OIDC issuer, bootstrap admin, workspace and project-files images, registry TLS mode |
| `noryx-frontend.yaml` | OIDC URL, frontend client ID, documentation and status links, frontend image tag |
| `keycloak.yaml` | Keycloak hostname and image tag |
| `noryx-api-ingressroute.yaml` | Public hostname and the TLS strategy |
| `noryx-backup-cronjobs.yaml` | Backup, PostgreSQL and Keycloak image placeholders |
| `noryx-environment-rebuild.yaml` | Registry destinations for every system environment |
| `noryx-mail-bridge.yaml` | Backend image tag; never leave it on `latest` |

Generate a different random value for every password and at least 32 random
bytes for `NORYX_SECRETS_MASTER_KEY`. This master key decrypts project secrets:
back it up in an access-controlled vault before users create secrets.

Render the complete installation manifest and stop if repository defaults
remain:

```bash
kubectl kustomize /path/to/private/ce-overlay > rendered-noryx-ce.yaml
./scripts/ops/validate-installation-manifest.sh rendered-noryx-ce.yaml
kubectl apply --dry-run=server -f rendered-noryx-ce.yaml
```

The validation deliberately rejects `example.local`, `change-me`, placeholder
backup images and `:latest`. A rendered manifest that fails this step is not
ready to apply.

## 5. Configure DNS and TLS

The public hostname must resolve to the ingress or external edge before OIDC
and TLS are configured. Expose these paths through the same hostname:

- `/` for the console;
- `/auth` for Keycloak;
- `/api` and `/swagger` for the API;
- `/workspaces` and `/apps` for authenticated workload proxies;
- `/healthz` for the deployment smoke test.

Choose one TLS owner:

- **Traefik:** configure the certificate resolver used by
  `noryx-api-ingressroute.yaml` and let Traefik terminate TLS.
- **External edge:** terminate TLS at HAProxy, Nginx or another edge and pass
  HTTP to Traefik. Do not leave Traefik configured as a second, competing TLS
endpoint.

The public certificate must cover the hostname used in the frontend OIDC
configuration and the backend `NORYX_OIDC_ISSUER_URL`.

## 6. Apply and wait for the control plane

Apply only the rendered manifest validated in the previous step:

```bash
kubectl apply --server-side --force-conflicts -f rendered-noryx-ce.yaml

kubectl -n noryx rollout status deployment/postgres --timeout=10m
kubectl -n noryx rollout status deployment/keycloak --timeout=10m
kubectl -n noryx rollout status deployment/noryx-backend --timeout=10m
kubectl -n noryx rollout status deployment/noryx-frontend --timeout=10m
kubectl -n noryx get pods
kubectl -n noryx-loads get pods
```

`Running` only proves that containers started. It does not prove OIDC, the
frontend configuration, the certificate or registered routes work.

## 7. Bootstrap and harden identity

After Keycloak is ready, create the realm and the first platform administrator.
Pass the passwords from the private secret store; do not paste them into shell
history or documentation.

```bash
NS=noryx \
ADMIN_USER=admin \
ADMIN_PASS='<keycloak-admin-password>' \
BOOTSTRAP_USER='<platform-admin>' \
BOOTSTRAP_PASS='<platform-admin-password>' \
BOOTSTRAP_EMAIL='admin@example.org' \
./scripts/keycloak/bootstrap-realm.sh

NS=noryx ./scripts/keycloak/harden-realm.sh
```

The hardening script enables brute-force protection, a password policy and
removes the direct login flow from the API audience client. Confirm the
frontend OIDC client has only the exact public redirect URI and web origin for
your hostname. See [Keycloak setup](KEYCLOAK_SETUP.md) for its operational
details and token test.

## 8. Run the deployment smoke test

The smoke test checks the served frontend and backend versions, TLS, OIDC
discovery, API behaviour and the registered surfaces. Run it through the same
edge users will reach.

```bash
BASE_URL="https://${NORYX_DOMAIN}" \
EXPECT_EDITION=community \
NAMESPACE=noryx \
./scripts/ops/smoke_deployment.sh
```

If the edge is not directly reachable from the administration workstation,
set `SMOKE_RESOLVE_IP` to test the edge IP while preserving the hostname and
SNI. Do not use `SMOKE_INSECURE=1` for a public deployment; it disables the
certificate verification the smoke test is supposed to prove.

## 9. Validate the first user journey

Log in as the bootstrap administrator and verify the workflow in this order:

1. Create a project.
2. Register a small non-sensitive S3 dataset and attach it to the project.
3. Attach a test Git repository.
4. Launch one workspace with a system environment.
5. Confirm the workspace opens and the project dataset appears under
   `/datasets`.
6. Stop the workspace, launch one job, and read its logs.
7. Publish a minimal application and confirm that its access rule is enforced.

This test proves more than a pod status: project RBAC, registry pulls, S3 CSI,
OIDC proxying and workload routing all participate in the journey.

## 10. Operate after day one

- Record the deployment commit, image digests, domain, registry and storage
  decisions in the private operations repository.
- Run the smoke test after every deployment.
- Use `kubectl diff -f rendered-noryx-ce.yaml` before an update, then apply the
  same rendered manifest used for review.
- Review [workload network isolation](WORKLOAD_NETWORK_ISOLATION.md) before
  granting broad outbound access from workloads.
- Configure and rehearse backup and recovery before calling the platform
  production-ready. A dataset bucket has its own backup responsibility; a
  platform metadata backup is not a copy of a terabyte of S3 objects.

## Troubleshooting and follow-up documents

- [Infrastructure prerequisites](INFRA_PREREQUISITES.md)
- [Image security and Harbor mirroring](IMAGE_SECURITY.md)
- [Keycloak setup](KEYCLOAK_SETUP.md)
- [Workspace troubleshooting](WORKSPACE_TROUBLESHOOTING.md)
- [Direct S3 dataset mounts](S3_DATASET_MOUNTS.md)
- [Observability](OBSERVABILITY.md)
- [Recovery](RECOVERY.md)
- [API contract](API.md)

When reporting an installation failure, include the CE commit SHA, the
rendered-manifest validation output, the smoke-test output and the affected pod
logs. Do not include passwords, bearer tokens, S3 credentials or dataset paths.
