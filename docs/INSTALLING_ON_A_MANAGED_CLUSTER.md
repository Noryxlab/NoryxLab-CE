# Installing on a managed cluster: EKS, GKE, AKS

What a client's own Kubernetes has to provide, what degrades if it does not,
and the one thing that needs code we have not written.

Written because the question arrives from sales, not from engineering, and the
answer was being reconstructed each time from reading the source. The figures
below were measured on a running installation on 26 November 2026, not
estimated.

## What is just Kubernetes

Postgres, Keycloak, the backend, the frontend, workspaces, jobs, applications
and scheduled jobs are ordinary Deployments, Jobs and CronJobs. Environment
builds run **Kaniko in the cluster** — no Docker daemon, no privileged build
node — and push to whatever registry the credentials point at.

Image digests are resolved through the **standard registry API**, including
the bearer token flow, so ECR, Artifact Registry, ACR, GitLab's registry and
Harbor are equivalent for provenance: a run records the digest that executed
and the pod runs that digest.

## The four things that decide

### 1. A ReadWriteMany storage class

`/mnt` (the project volume) and the per-person profile volume are **RWX**. A
project's workspace, job and application run at once and share them.

The block storage of all three clouds — EBS, Persistent Disk, Azure Disk — is
RWO. Installed against one, workspaces stay `Pending` with no message anybody
can read. So the cluster needs a shared-filesystem class:

| | class |
| --- | --- |
| EKS | EFS CSI |
| GKE | Filestore CSI |
| AKS | Azure Files |

Both classes are configurable and default to `longhorn`:
`NORYX_WORKSPACE_PVC_STORAGE_CLASS` and
`NORYX_WORKSPACE_PROFILE_PVC_STORAGE_CLASS`.

**What this costs, measured.** On-cluster RWX (Longhorn) serves the volume
through an NFS server pod — `/mnt` is literally an `nfs4` mount. Against the
pod's local disk:

| | `/mnt` (RWX) | local disk | ratio |
| --- | --- | --- | --- |
| sequential write, 256 MiB | 792 ms | 283 ms | 2.8× |
| sequential read, 256 MiB | 31 ms | 25 ms | ~1× |
| **2000 small files** | **3 406 ms** | 131 ms | **26×** |
| listing 2001 entries | 20 ms | 4 ms | 5× |

Throughput is fine; **metadata operations are not**, and a Python environment
is metadata. Installing numpy, scipy, pandas, scikit-learn, matplotlib and
friends — 18 929 files — takes 66 s on the shared volume against 21 s on local
disk.

A managed shared filesystem is the same shape with **more** latency, because
it is a service across the network rather than a pod on the same node. Plan
for it rather than discovering it: keep what writes thousands of small files
off the shared volume.

### 2. Node pools that can run a FUSE DaemonSet

Datasets are mounted with the `ru.yandex.s3.csi` driver, a privileged
DaemonSet mounting FUSE. That rules out **GKE Autopilot** and **EKS Fargate**.
Standard node pools on all three clouds are fine. This is the constraint to
raise first, because it eliminates whole architectures rather than costing
money.

### 3. NetworkPolicy that is actually enforced

Everything the platform says about regulated data resting in its perimeter is
enforced with NetworkPolicy. A cluster whose CNI does not enforce them
**accepts every rule and applies none**, with nothing to see from inside.

| | what to check |
| --- | --- |
| EKS | VPC CNI ≥ 1.14 with network policy enabled, or Calico |
| GKE | Dataplane V2, or network policy enabled |
| AKS | a policy engine chosen **at cluster creation** — it cannot be added later |

Verify it on the cluster before the first regulated dataset is attached. This
is the worst failure mode in this document: silent, and on the part we sell.

### 4. Object storage that speaks S3

A dataset is a bucket, a prefix and a credential, reached with an S3 client.

- **AWS S3**: native.
- **GCS**: through the interoperability endpoint with HMAC keys.
- **Azure Blob**: **no S3 API at all.** An installation on Azure runs, with
  datasets on the in-cluster MinIO or on any S3-compatible endpoint the client
  already has — but "mount our existing Blob containers" is not possible
  today.

Supporting Blob natively is two unequal pieces. The **mount** is one branch at
the CSI volume spec to use Microsoft's blobfuse2 driver — a day. The
platform's own reads — explorer, previews, extracts, ontology scan, snapshots,
fingerprint crossing — go through `internal/objectstore`, which is an
interface with one implementation today; a second one is one to two weeks.
That package exists for exactly this.

## What degrades, and nobody notices unless it is written down

Neither of these stops an installation. Both are things somebody will
eventually ask about, and the answer should not be a surprise.

**Registry enrichment, without Harbor.** Image sizes in the environment
catalogue, vulnerability counts (Harbor scans with Trivy on push) and deleting
a built environment image all use Harbor's own API, which the registry
standard does not cover. Provenance is unaffected.

**The storage headroom alert, without Longhorn.** It warns before a cluster
can no longer place a volume — a claim-based figure, not free disk, and the
difference is the whole point: a cluster with 78% of its disk empty can be
unable to place one more volume, which once cost two people a morning. It
reads Longhorn's API, and on a cloud CSI it reports "unmeasurable" and stays
quiet. Deliberate — "critical" about a reading that does not exist teaches an
operator to ignore the alert — but it means a cloud installation has no
advance warning of that particular wall.

## Ingress

`EDGE_TLS_MODE=ingress` renders the routing as a standard
`networking.k8s.io/v1` Ingress, for a cluster whose controller is not Traefik.
The class is left to the installation. The two Traefik modes stay for the k3s
installations that use them.

## What the Ansible bootstrap is for

`ansible/playbooks/bootstrap-demo.yml` installs k3s, Longhorn and the platform
on a bare Debian host. On a managed cluster it has no role: the cluster
already exists, and the manifests are applied directly — simpler, not harder.
