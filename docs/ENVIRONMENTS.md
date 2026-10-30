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
- optional `dockerfileContent` (inline edited content)
- `contextPath`
- `status`

The environments API aggregates build history by `(projectId, destinationImage)`.

## Current behavior

- project-scoped visibility: caller only sees environments for projects where caller has a role
- revisions are sorted by creation time (newest first)
- latest revision metadata is exposed directly on each environment item
- Dockerfile content can be fetched per revision (build ID)
- recommended registry namespace for environment images: `harbor.example.local/noryx-environments/...`

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
- select an environment revision and load its Dockerfile
- edit Dockerfile inline
- submit a new build revision directly from edited Dockerfile content

## The base image

A Dockerfile written in the environment screen may name a platform
environment without its registry — `FROM noryx-jupyter`, and `noryx-python`,
the name the Jupyter image used to carry. The platform rewrites those to the
image this installation is configured to run, keeping a tag chosen on purpose
and the `FROM`'s own options, and stores the resolved reference. A reference
that already names a registry, or an image the platform does not own such as
`python:3.12-slim`, is left exactly as written.

Without that, a bare name goes to Docker Hub: every environment started from
the blank template failed with "authentication required" against a registry
nobody had asked for.

## Building a revision

A revision is a record, not a state: an environment whose only revision
failed keeps showing that failure until a new revision is built. Nothing
replays a build.

From the environment list, the row menu offers *build a revision*, which opens
the panel on the environment's definition with its build project already
filled in — the project that built the previous revision. Creating an
environment still asks for the project explicitly and never defaults it,
because a build spends that project's quota and is attributed to it.

A failed revision keeps its reason. The build pod is deleted with its job
within the day, so the last error line is copied into the record when the
status is read, and shown beside the verdict in the revision list.

## Notes and limits

- Dockerfile fetch currently supports public repositories on:
  - `github.com`
  - `gitlab.com`
- if a revision was created with inline Dockerfile content, `GET /api/v1/builds/{buildID}/dockerfile` returns this stored inline content
- build list and status are synced from Kubernetes jobs (`app.kubernetes.io/name=noryx-build`)
- runtime namespace for jobs: `NORYX_WORKLOAD_NAMESPACE` (current deployment: `noryx-loads`)
