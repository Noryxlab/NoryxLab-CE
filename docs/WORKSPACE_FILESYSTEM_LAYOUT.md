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
- user profile directory: `/home/noryx/.noryx-profile`

## Persistence model

- `/mnt`: persistent at project scope (shared project PVC)
- `/repos`: ephemeral at workspace scope
- `/home/noryx/.noryx-profile`: persistent at user scope
- `/datasets`: dataset mounts managed by Noryx dataset flow

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
