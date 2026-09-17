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
Cloud Cellar, outside the site entirely, and Clever backs those buckets up on
their side as well. Nothing that matters is stored only on the machine it
protects, and the copy that leaves the site is itself copied.

**What no number of copies covers: a deletion made with valid credentials.**
Every layer above answers the question "what if something breaks". None of them
answers "what if somebody with the key removes it on purpose, and the removal
replicates". The protections against that are different in kind - object
versioning or an object lock on the bucket, and a credential that cannot delete
- and they are worth checking rather than assuming, because the arrangement
otherwise looks complete enough that nobody looks.

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
./scripts/ops/rehearse-restore.sh                       # data, into a throwaway database
REHEARSE=1 ./scripts/ops/restore-identity.sh <object>   # accounts, likewise
./scripts/ops/rehearse-volume-restore.sh                # a volume, newest backup
./scripts/ops/rehearse-volume-restore.sh backup-ea40e83a4e7741ae  # a specific one
```

All three leave the live platform untouched and remove what they created. Run
them on a schedule: the first time anybody restores must not be the day it
matters.

The volume rehearsal restores a Longhorn backup into a volume of its own and
compares it to the live one file by file - an MD5 over each file's MD5, sorted
by path, because a count of files or bytes would pass on a volume restored with
the right shape and the wrong contents. Where the source claim still exists it
is mounted read-only as the reference; where it does not, the restore is
reported without a comparison rather than silently claiming success.

Two things it is careful about, both learned by getting them wrong:

- **Cleanup is a trap, not a final line.** A run that fails halfway would
  otherwise leave a ten-gigabyte volume behind for somebody to find later and
  not know whether it mattered.
- **A difference is not automatically a bad backup.** If the volume was written
  to after the backup was taken, the live copy is no longer a valid reference.
  The script says so instead of reporting a failure, and prints the time the
  backup was taken so the question can be settled.

## Rehearsal, 2026-09-17

A Longhorn backup was restored and checked file by file, on the DC.

| | Live volume | Restored |
|---|---|---|
| Files | 10,722 | 10,722 |
| Bytes | 439,799,375 | 439,799,375 |
| MD5 over the per-file checksums | `96219e116ca76b701b7c2a73f38b346d` | `96219e116ca76b701b7c2a73f38b346d` |

The volume was a project workspace holding a Python environment and its
dependencies - ten thousand small files, which is a harder case than one large
one. The night's backup was taken at 00:40 and nothing on the volume had been
written since, so the live copy was a valid reference: any difference would
have been a defect rather than drift.

Restoring took thirty seconds. The restored volume was mounted in a throwaway
pod, fingerprinted, and destroyed; nothing in production was touched, and the
live volume was only ever mounted read-only.

**Why it was worth doing now rather than trusting the previous one.** The last
rehearsal was on 2026-09-06, and two things in this path have changed since:
Longhorn backups were added on the DC on 2026-09-16, and the object store's
credentials were rotated on 2026-09-17. Both were verified to the extent that a
backup completed afterwards - which proves a file was written, not that it can
be read back. This proves it can.

All three paths were then rehearsed the same evening, on the DC:

| Path | Result |
|---|---|
| Longhorn volume | 10,722 files, 440 MB, identical to the byte |
| Database | HTTP 200, nine tables match; the backup holds twelve projects against eleven live, because one was deleted after it was taken |
| Identity | 24 accounts, all with a password hash, and 7 organizations - the same 24 the platform has live |

The database drill did not work when it was first run, and the three defects it
had are worth keeping in mind for the next script of this kind. It had no
credential and fell back to a header the platform has refused since component
tokens landed. It reported that as "no backup object to restore", which sent
the reader to look for a missing backup that was never missing. And the restore
request went out before the throwaway backend was listening - `rollout status`
returns when a container is running, and running is not listening.

The worst of the three was none of those: with the restore failed, the drill
printed nine CHECK lines and exited 0. A rehearsal that reports success while
proving nothing is the failure this document exists to prevent, occurring in
the instrument meant to detect it.

What remains unproven, and is written here so it is not mistaken for proven:
the same three exercises on EMSE.
