# Recovering the platform after losing the cluster

This is the order, and it is short on purpose. It was written by rehearsing
each step against the real backups on 2026-09-06, not from memory.

## Three layers, and which one you want

Losing "the platform" means different things, and the answer is a different
backup each time. Written down because the one nobody remembers under pressure
is the one that would have been quickest.

| Lost | Restore from | What comes back |
|---|---|---|
| A volume, or what was in it | **Longhorn → Cellar** | One volume, as of the last nightly job |
| Projects, rights, accounts, audit | **Database and identity backups → Cellar** | The platform's own state, into a cluster that already runs |
| The machine: OS, k3s, configuration | **Proxmox Backup Server** | The whole virtual machine, disks included |

They are complementary rather than ranked. Restoring a VM from PBS brings back
a cluster with its volumes attached but tells you nothing about a file somebody
deleted last Tuesday; restoring a Longhorn volume brings that back but not a
machine that no longer boots. The rest of this document is the second row,
which is the one with the most moving parts.

**Where each one lives, on the EMSE installation.** PBS writes to the EMSE
DSI's storage - off `kvm-premyom`, so losing that host or its disk does not
take the backups with it. Longhorn and the platform backups write to Clever
Cloud Cellar, outside the site entirely. Nothing that matters is stored only on
the machine it protects.

**A PBS snapshot of a running machine is crash-consistent, not
application-consistent.** It captures Longhorn volumes mid-write. What comes
back is a filesystem that needs to repair itself and replicas Longhorn may have
to rebuild - a real backup, and not the same thing as a volume backup taken by
the storage layer that owns the volume. That is precisely why both exist.

**The DC has only the first two layers.** Its Proxmox carries no PBS datastore,
so there is no machine-level restore for `noryxlab-master`: losing that VM
means rebuilding it and restoring into it. Acceptable for a sandbox, worth
knowing before treating it as anything more.

## What must exist outside the cluster

Two things. Everything else is inside the backups, encrypted.

1. **The backup target's access key and secret.** They cannot travel inside
   the object they are needed to fetch. On this installation the target is
   Clever Cloud Cellar, so they can also be reissued from that account.
2. **`NORYX_RECOVERY_KEY`.** Without it the credential bundle is noise.

Keep both where the Keycloak administrator password is kept. Not in Noryx: to
read a secret stored in Noryx you would need a restored Noryx, and to restore
Noryx you need these.

A third, optional: **`NORYX_IDENTITY_BACKUP_KEY`**. It is inside the credential
bundle, so keeping it separately is belt and braces rather than a requirement.

## Before any of this: the target has to exist

An installation with no `noryx-backup-target` secret still schedules backup
runs. The scheduler records them, nothing is ever written, and the platform
looks backed up because something is on the calendar. EMSE ran that way from
its installation until 2026-09-06: 34 scheduled runs, zero stored bytes.

```sh
ENDPOINT=https://cellar-c2.services.clever-cloud.com BUCKET=noryx-<site> \
ACCESS_KEY=... SECRET_KEY=... \
  ./scripts/ops/configure-backup-target.sh noryx
```

The script writes and deletes a probe object from inside the cluster before it
stores anything, so credentials that only work from a laptop are refused. A
bucket inside the cluster is refused too, unless `ALLOW_IN_CLUSTER_TARGET=1`:
it covers a dropped table and covers nothing on the day the cluster is lost.

## The order

**1. Open the credential bundle.** Needs a machine with `docker`, or with `mc`
and `openssl`. On this installation the build VM has docker and the cluster
master does not, so extraction and application are separate steps.

```sh
EXTRACT_TO=~/recovered NORYX_RECOVERY_KEY=... \
BACKUP_ACCESS_KEY=... BACKUP_SECRET_KEY=... \
  ./scripts/ops/restore-credentials.sh recovery/<date>/credentials.tar.enc

# then, from a machine with kubectl
./scripts/ops/restore-credentials.sh --apply ~/recovered noryx
```

The platform now has its master key, its backup target, its registry
credentials and its service token - and can read everything else itself.

**2. Deploy the platform.** The data tier first (`postgres.yaml`, `minio.yaml`,
`keycloak.yaml` rendered for the installation), then the Enterprise deploy
script. Check `PGDATA` against what the old deployment used: the DC ran with
none and EMSE with `/var/lib/postgresql/data/pgdata`, and getting it wrong runs
`initdb` beside the real cluster.

**3. Restore the data.**

```sh
POST /api/v1/admin/backups/runs/external/restore
  {"mode":"missing-only","objectKey":"<date>/<run>/manifest.json"}
```

Projects, datasets, data sources, ontologies, apps, repositories and grants.
Jobs, workspaces and builds are deliberately not restored: they describe what
was running, not what the platform is.

**4. Restore the accounts.**

```sh
./scripts/ops/restore-identity.sh identity/<date>/noryx-identity.tar.enc noryx
```

Users with their password hashes, organizations and memberships. It replaces
the realm, so it refuses while one already exists unless
`ALLOW_REALM_OVERWRITE=1` says otherwise.

**5. The audit trail, if it is needed.** `audit.ndjson.gz` sits beside each
manifest. It is not replayed automatically: importing a history into a platform
that already has one interleaves two of them, so it is an operator's decision.

## What is not recovered by any of this

- **Object storage contents.** A declared boundary: dataset buckets are
  protected by storage replication, not by being copied into a backup.
- **Container images.** They live in Harbor, on its own machine. If Harbor is
  lost too, images have to be rebuilt from the repositories - which is why
  runs record an image digest rather than only a tag.
- **What a dataset contained at a point in time.** The platform records which
  dataset was attached to a run, not its contents. True data versioning needs
  immutable snapshots and has not been decided.
- **Project and profile volumes that are not attached.** The nightly Longhorn
  job backs up the volumes that have a running engine - the ones a workspace is
  using. A volume whose last workspace stopped is skipped.

  This is a decision, taken 2026-09-16, and it follows from what a workspace
  is: **a disposable unit.** Code belongs in a repository, data belongs in
  object storage, and a project volume is the scratch space in between. Backing
  up active work and not dormant scratch space is not a gap in the policy, it
  is the policy - and a nightly job that reattached every dormant volume in the
  installation would buy coverage with I/O and with a moving part that fails
  quietly.

  What this asks of the people using the platform is therefore explicit:
  **commit and push.** Anything that exists only inside a stopped workspace is
  not backed up by the platform and is not meant to be. A workspace can be
  reaped at its maximum lifetime, a volume can be lost with its node, and
  neither event should cost anybody more than the time to clone a repository
  and relaunch.

  The figure to check rather than assume: on the DC on 2026-09-16 the nightly
  job completed five backups against fourteen volumes - five attached, nine
  detached. A report that says "backups completed" without saying against how
  many volumes is the sentence this paragraph exists to prevent.

## Rehearsing

None of this is believable until it has been run.

```sh
./scripts/ops/rehearse-restore.sh            # data, into a throwaway database
REHEARSE=1 ./scripts/ops/restore-identity.sh <object>   # accounts, likewise
```

Both leave the live platform untouched and remove what they created. Run them
on a schedule: the first time anybody restores must not be the day it matters.
