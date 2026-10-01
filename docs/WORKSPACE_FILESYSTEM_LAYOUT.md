# Workspace Filesystem Layout (Target Contract)

This document defines the target filesystem contract for interactive workspaces.

Status:

- contract is validated
- runtime baseline is implemented in CE
- operational details are documented in `docs/WORKSPACES.md`

## Paths

- project work directory: `/mnt`
- project requirements file: `/mnt/requirements.txt`
- repositories directory: `/repos`
- datasets mount root: `/datasets`
- extracts mount root: `/extracts`
- user profile directory: `/home/noryx/.noryx-profile`

## Persistence model

- `/mnt`: persistent at project scope (shared project PVC)
- `/repos`: ephemeral at workspace scope
- `/home/noryx/.noryx-profile`: persistent at user scope
- `/datasets`: dataset mounts managed by Noryx dataset flow
- `/extracts`: rebuilt at every start, from the extracts the project attaches

`/extracts` holds a tree of symlinks into `/datasets`, arranged the way each
extract asked for - `<name>/<levels>/<file>`, where the levels are the order
the extract declared. Nothing is copied: the bytes stay in the dataset mount,
which is read-only.

It sits beside `/datasets` rather than inside `/mnt` for two reasons. An
extract is a read-only view of a bucket, so it belongs beside the bucket; and
the tree is rebuilt with `rm -rf` at every start, which has no business
happening inside the volume the project writes to - anybody who had made their
own `/mnt/extracts` would have lost it.

In the isolated mode (ADR-038) the same path is a read-only volume filled by a
container beside the workspace, and the datasets are mounted nowhere the person
can reach. The path is the same either way, which is what lets a notebook move
between the two without being edited.

## Runtime user

- default user: `noryx`
- `sudo` enabled for `noryx`

## Concurrency

One user can run multiple workspaces at the same time.

Implication:

- user profile storage must support concurrent read/write (RWX-capable storage backend)

S3 note:

- S3/object storage is not used as direct live filesystem for IDE profile directories
- S3 can be used as backup target for volume snapshots/backups

## IDE behavior target

- Jupyter starts in `/mnt`
- VSCode default folder is `/mnt`
- optional dependency bootstrap checks `/mnt/requirements.txt` at startup

## Project file explorer

The UI file explorer accesses the project PVC through an on-demand technical
`project-files-<project-id>` pod. It is intentionally separate from workspace
pods so files remain accessible when no workspace is running and so multiple
workspaces do not become ambiguous proxy targets.

The technical pod:

- is excluded from user workload metrics;
- exits after 15 minutes without file operations;
- is deleted and recreated automatically on the next file explorer request.
