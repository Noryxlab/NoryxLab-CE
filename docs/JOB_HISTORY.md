# Job history and results

The Jobs page keeps a persistent execution history per project.

- Immediate jobs and executions created by scheduled jobs are discovered from
  Kubernetes.
- Status, launch time and completion time are synchronized into PostgreSQL.
- Up to 2,000 final log lines are captured as the persistent job result when an
  execution succeeds or fails.
- Once captured, the result remains available after the Kubernetes Job and pod
  are cleaned up.
- While a job is starting or running, the Logs action stays on the Jobs page.
  If the pod or container is not ready yet, the UI displays an informational
  message instead of redirecting to Administration.
- Explicitly deleting a job deletes both its Kubernetes resources and its
  persisted history entry.

## What a job read

A job records the datasets it had mounted at the moment it started —
identifier, name, bucket, prefix, and whether it could write. Until this
existed, a run recorded the image digest that produced it and nothing about
the data that went in, so in six months nothing would say which snapshot of an
extract a calculation read.

Since 2026-10-01 it also records **which extracts it read**, by name, for the
same reason exactly: a project's attachments change and a result does not. The
datasets answer "which bucket"; the extracts answer "which files", and only the
second is reproducible — an extract is a frozen list, so naming it is naming
the files, where naming a dataset names a bucket that has grown since.

A job can now mount extracts at all, which it could not before: only a
workspace could, so a calculation wanting the same n had to mount the whole
dataset and re-filter in its own code. That is the thing an extract exists to
remove, and recording what was mounted is worth little while what was mounted
could not be the selection.

Two details are worth knowing, because both are the difference between a
record and a guess:

- **The status watcher upserts without knowing what was read**, so an empty
  list never overwrites a recorded one. A job updated by the watcher keeps the
  mounts recorded at launch.
- **A job with nothing attached records an empty list rather than null.**
  "Read no data" stays distinguishable from "nobody wrote it down" — which is
  what every run before this reads as, and pretending otherwise would be worse
  than an absence that says so.

Nothing is backfilled. The runs that already happened cannot be reconstructed.

This is the first half of lineage. The second — the artefact a job produced,
and the model that names it — is not built: a model that produces an optical
design that becomes a medical device raises the question "which version, fed by
which snapshot, produced this", and answering it is a deliberate piece of work
rather than a field to add here.
