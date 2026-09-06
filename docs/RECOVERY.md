# Recovering the platform after losing the cluster

This is the order, and it is short on purpose. It was written by rehearsing
each step against the real backups on 2026-09-06, not from memory.

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

## Rehearsing

None of this is believable until it has been run.

```sh
./scripts/ops/rehearse-restore.sh            # data, into a throwaway database
REHEARSE=1 ./scripts/ops/restore-identity.sh <object>   # accounts, likewise
```

Both leave the live platform untouched and remove what they created. Run them
on a schedule: the first time anybody restores must not be the day it matters.
