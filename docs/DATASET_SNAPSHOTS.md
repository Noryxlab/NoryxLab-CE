# Dataset snapshots

A snapshot records what a dataset held at an instant — path, size, last
modification, ETag — and **copies nothing**. It is also the only way to
enrol a dataset in the nightly drift check.

The button is on the dataset's page in the catalogue. The decision is
`ADR-058`, the companion to the co-mount rule in `ADR-055`.

## It is not a copy, and that matters

These are regulated datasets. Doubling regulated bytes creates a second place
to govern, find and delete them, so what the platform keeps is a manifest and
the bytes stay where they are. Listing a bucket costs one request per thousand
objects and reads no content, which is what makes the operation acceptable on
health data and why it is open to whoever can already read the dataset.

Copying a dataset is a separate act behind a separate capability,
`copy-dataset`, which nobody holds. **The capability is declared and the
operation is not written.**

A snapshot is therefore **not a backup**. A manifest cannot restore anything.
The word invites that misreading, which is why the screen says "not a copy"
before it says anything else.

## What it answers

"Has this file changed since?" — a question nothing else on the platform could
settle. An ontology describes a dataset's shape; an extract freezes a selection
without duplicating a byte. Neither records a fingerprint, so a dataset could be
rewritten under a finished study and nothing would notice.

That matters for two reasons at once:

- **Audit.** A write through a mounted bucket does not cross the API, and the
  API is the only place writes are recorded. Comparing a manifest with the
  bucket as it stands is the only thing that sees it.
- **Reproducibility.** [A replay](REPLAYING_A_RUN.md) names its datasets; it
  cannot freeze them. A snapshot is how you find out whether the data moved
  under a result.

## The nightly check

Every night, each dataset **that somebody has snapshotted** is compared with
its latest snapshot, and any difference is reported to the administrators — up
to ten paths named, then "and N others", because an alert that unrolls four
thousand paths is not read.

Only the snapshotted ones. Fingerprinting every bucket each night would mean
listing all of the platform's data to answer a question nobody asked about most
of it. **The snapshot is how an installation says "this one, I want to know".**

It follows that a dataset leaves the check when its last snapshot is deleted,
and that there is no other way to un-enrol it.

> The check currently runs every 24 hours counted from the backend's start, not
> at a fixed hour, so a rollout moves the time it runs.

**A second check shares the same pass**, and does not need a snapshot: the
fingerprint crossing of `ADR-060` looks for objects of a regulated dataset
reappearing in an unclassified dataset attached to the same project. It is the
counterpart of the `write-beside-regulated-data` capability, it reads live
listings rather than manifests, and it is described in
[how data leaves a workspace](HOW_DATA_LEAVES_A_WORKSPACE.md).

## Comparing

*Show the differences* lists what changed since a snapshot, as `added`,
`removed` or `modified`, with the path and the sizes on both sides of a
modification. It lists the whole bucket when you ask for it and not before,
which on a dataset of a hundred thousand objects is a minute of work nobody
requested.

**An ETag says "these differ", never "these are identical".** An ETag is not a
portable checksum: a multipart upload produces a different one for the same
bytes. A matching size with a different ETag is therefore reported as
*modified* — the honest answer is that something changed, not that nothing
did.

## Who may do what

| Act | Access needed |
| --- | --- |
| Take a snapshot | read on the dataset |
| Compare one | read |
| Delete one | **write** |

The asymmetry is deliberate. Taking a snapshot learns nothing a listing does not
already give somebody who can read the dataset. Deleting one is the opposite
act: a dataset's last snapshot is what enrols it in the nightly check, so
removing it stops a control. The refusal says that in full rather than answering
"forbidden" to somebody who was allowed to create the thing they cannot remove.

## Limits

A snapshot needs a dataset with a bucket, and it is read **through that
dataset's own storage endpoint and credential** — including an external S3
endpoint, which is where the regulated datasets live.

It does not prevent a write. A researcher with write access to a dataset is
allowed to write to it; this stops ignoring it. And it does not see a write and
a revert that both happen between two scans.
