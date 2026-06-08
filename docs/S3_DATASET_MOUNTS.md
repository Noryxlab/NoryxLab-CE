# Direct S3 dataset mounts

Noryx mounts every S3-backed dataset directly into project workloads. Objects
are not copied into workspace ephemeral storage or project PVCs.

This architecture applies to:

- standard S3 datasets in CE and EE
- regulated HDS datasets in EE
- workspaces, jobs, applications, and dashboards

## Runtime flow

When a workload starts, the backend:

1. resolves datasets attached to the selected project;
2. checks the caller's dataset and organization RBAC;
3. creates or reuses one Kubernetes Secret, PV, and PVC per dataset;
4. mounts each PVC under `/datasets/<dataset-name>`;
5. starts the workload without passing S3 credentials in its bootstrap script.

The PV uses the `ru.yandex.s3.csi` driver with GeeseFS. The PVC capacity shown
by Kubernetes is a nominal `1Gi`; it does not represent or limit the bucket
size.

## Access control

- `owner` and `writer` datasets are mounted read/write.
- `reader` datasets are mounted read-only.
- HDS datasets require EE, the HDS feature gate, and the corresponding dataset
  or organization permission.
- Each external dataset uses dedicated encrypted S3 credentials. There is no
  shared credential fallback.
- CSI credentials are stored in a dedicated Secret in the workload namespace,
  not in the workload bootstrap script.

Dataset access must still be restricted by the S3 provider policy. Kubernetes
and Noryx RBAC do not replace bucket-side least-privilege permissions.

## Lifecycle

The dataset PV, PVC, and Secret are reused across workloads. Stopping a
workspace does not remove them.

Deleting a dataset through Noryx removes:

- its Kubernetes PVC;
- its Kubernetes PV;
- its CSI credentials Secret;
- its encrypted platform credential record.

Deleting a dataset registration does not delete objects from the external S3
bucket.

## Installation

Install the pinned CSI driver version:

```sh
scripts/ops/install-s3-csi.sh
```

The backend service account requires CRUD access to workload-namespace Secrets
and PVCs, and cluster-scoped PVs. The required rules are defined in:

```text
deploy/k8s/base/noryx-api-rbac.yaml
```

Verify the installation:

```sh
kubectl get csidriver ru.yandex.s3.csi
kubectl -n kube-system get pods -l app=csi-s3
```

## Operational checks

List managed dataset volumes:

```sh
kubectl -n noryx-loads get pvc,secret \
  -l app.kubernetes.io/name=noryx-dataset-volume
kubectl get pv -l app.kubernetes.io/name=noryx-dataset-volume
```

Verify mounts in a workload:

```sh
kubectl -n noryx-loads exec <pod> -- mount | grep /datasets
```

Expected filesystem type:

```text
fuse.geesefs
```

Verify that the bootstrap contains no legacy copy or synchronization logic:

```sh
kubectl -n noryx-loads exec <pod> -- \
  grep -E 'from minio|initial_sync|fget_object|fput_object' \
  /var/run/noryx/bootstrap/bootstrap.sh
```

The command must return no match.

## S3 filesystem semantics

An S3 mount is not a fully POSIX filesystem:

- object renames can require copy/delete operations;
- metadata and directory listings may be cached;
- many small-file operations are slower than local or block storage;
- concurrent writers must be designed with S3 consistency semantics in mind;
- applications requiring databases, file locks, or strict POSIX behavior must
  use the project PVC instead.

Use `/datasets` for object data and `/mnt` for project files requiring normal
filesystem semantics.

## Troubleshooting

If a workload remains pending:

```sh
kubectl -n noryx-loads describe pod <pod>
kubectl -n noryx-loads get events --sort-by=.lastTimestamp
kubectl -n kube-system logs -l app=csi-s3 -c csi-s3 --tail=200
```

Common causes:

- invalid endpoint, bucket, or credentials;
- bucket-side policy denying list/read/write operations;
- CSI driver unavailable on the target node;
- missing backend RBAC for Secrets, PVCs, or PVs;
- a `reader` dataset used by a workload that attempts to write.
