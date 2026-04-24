# Environments Module (CE)

This document describes the current CE environment model and API.

## Model

Noryx CE treats an environment as:

- one target image (`destinationImage`)
- plus a list of build revisions (Kaniko jobs)

Each revision is linked to:

- `gitRepository`
- `gitRef`
- `dockerfilePath`
- `contextPath`
- `status`

The environments API aggregates build history by `(projectId, destinationImage)`.

## Current behavior

- project-scoped visibility: caller only sees environments for projects where caller has a role
- revisions are sorted by creation time (newest first)
- latest revision metadata is exposed directly on each environment item
- Dockerfile content can be fetched per revision (build ID)

## Endpoints

- `GET /api/v1/environments`
- `GET /api/v1/builds`
- `POST /api/v1/builds`
- `GET /api/v1/builds/{buildID}/dockerfile`

Optional query filter:

- `projectId` on `GET /api/v1/environments` and `GET /api/v1/builds`

## UI (current CE)

In the `Environments` tab:

- submit a new build (repository/ref/dockerfile/context/destination)
- list environment entries (name, image, latest status, revision count)
- inspect selected environment JSON details
- fetch and display Dockerfile for latest revision

## Notes and limits

- Dockerfile fetch currently supports public repositories on:
  - `github.com`
  - `gitlab.com`
- build list and status are synced from Kubernetes jobs (`app.kubernetes.io/name=noryx-build`)
- runtime namespace for jobs: `NORYX_WORKLOAD_NAMESPACE` (current deployment: `noryx-loads`)

