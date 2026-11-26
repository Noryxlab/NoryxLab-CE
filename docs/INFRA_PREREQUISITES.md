# Infrastructure Prerequisites (Preamble)

Noryx CE bootstrap depends on two external VMs:

- `harbor` VM: private container registry
- `dockerbuild` VM: image build and push runner

Without these two VMs, control-plane image pulls (`noryx-backend`, `noryx-frontend`) will fail in Kubernetes.

For a cluster the platform does not install - EKS, GKE, AKS or any other
managed Kubernetes - the prerequisites are different and are listed in
[installing on a managed cluster](INSTALLING_ON_A_MANAGED_CLUSTER.md): a
ReadWriteMany storage class, node pools that can run a FUSE DaemonSet, an
enforced NetworkPolicy implementation, and S3-compatible object storage.

## Any OCI registry, not only Harbor

The platform resolves the digest a tag points at through the **standard
registry API** (`/v2/.../manifests/...`, including the bearer token flow), so
ECR, Artifact Registry, ACR, GitLab's registry and Harbor are all equivalent
for the part that matters: a run records the digest that actually executed,
and the pod runs that digest.

Three things remain specific to Harbor, because the registry standard does not
cover them. They degrade; nothing fails:

- the **size** of an environment image, shown in the catalogue;
- **vulnerability counts**, which Harbor produces by scanning with Trivy on
  push;
- **deleting** an environment image built by the platform.

Image builds themselves are registry-agnostic: they run Kaniko in the cluster
and push wherever the credentials point.

## In-cluster storage baseline

Noryx CE bootstrap installs Longhorn CSI for workspace PVCs.

Node prerequisites (handled by Ansible `common` + `longhorn` roles):

- `open-iscsi` package
- `iscsid` service enabled
- sufficient local disk capacity for workspace volumes

## Required baseline

### Harbor VM

- hostname/IP reachable from k3s node
- Harbor registry installed and running
- project created: `noryx-ce` (private recommended)
- project created: `noryx-environments` (private recommended)
- robot account with `repository:pull` and `repository:push`
- tooling baseline: `jq`, `rsync`, `python3`
- mirrored essential images available in project:
  - `noryx-backend`
  - `noryx-frontend`
  - `postgres`
  - `keycloak`
  - `minio`
- mirrored environment base images available in project `noryx-environments`:
  - `noryx-jupyter`
  - `noryx-vscode`
  - `noryx-rstudio`

### Dockerbuild VM

- Docker installed and running
- network access to Harbor
- DNS/hosts resolution for Harbor hostname (for example `harbor.example.local`)
- credentials to push in `noryx-ce` project
- tooling baseline: `jq`, `rsync`, `python3`

## Quick setup scripts

Use provided scripts on each VM:

- `scripts/vm/install-harbor-vm.sh`
- `scripts/vm/install-dockerbuild-vm.sh`
- `scripts/ops/sync-images-to-harbor.sh` (run from dockerbuild after `docker login harbor.example.local`)

These scripts install required tooling for automation/debug:

- `jq` for JSON/API inspection
- `rsync` for fast repo sync to build hosts
- `python3` for lightweight operational scripts
- run sync for both catalogs:
  - `./scripts/ops/sync-images-to-harbor.sh`
  - `CATALOG_FILE=deploy/images/environments-images.txt ./scripts/ops/sync-images-to-harbor.sh`

## Kubernetes node alignment

The k3s node must be able to:

- resolve Harbor hostname
- trust Harbor TLS (or temporary `insecure_skip_verify` in lab mode)
- pull with `imagePullSecrets` (`harbor-regcred`)

If using split namespaces (`noryx-ce` + `noryx-loads`):

- create `harbor-regcred` in both namespaces
- builds/workspaces run in `noryx-loads`, so missing secret there breaks runtime launches
- ensure the cluster CNI enforces Kubernetes NetworkPolicy for user workload isolation

Workload isolation is documented in `docs/WORKLOAD_NETWORK_ISOLATION.md`.

## DNS note for in-cluster Kaniko builds

Kaniko build pods resolve registry hostnames using cluster DNS (CoreDNS), not host `/etc/hosts`.

If Harbor is exposed as `harbor.example.local`, add it to CoreDNS NodeHosts:

- `CHANGE_ME harbor.example.local`

Otherwise build jobs can fail with `lookup harbor.example.local on 10.43.0.10:53: no such host`.
