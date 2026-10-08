# Bulk dataset import

> Sending a study from your own machine? [PUSHING_TO_A_DATASET.md](PUSHING_TO_A_DATASET.md)
> is the one-page version. This page is the reference.

Use this path for a clinical import such as PREMYOM-1000 or SELENA: thousands
of files, tens of GiB, and a sender outside the platform network.

## Security model

Noryx authenticates the sender and checks their `writer` access to exactly one
dataset. It returns fifteen-minute S3 URLs, each for exactly one key that the
**server** composes from the dataset's own prefix — a path that tries to climb
out with `../` lands back inside it. The file bytes then travel directly from
the sender to the configured object store; Noryx never exposes the S3 access
key or proxies the health data.

The sender declares each object's size when asking, so the platform's own
object ceiling still applies. Without that declaration a presigned PUT passes
through none of the checks the proxied upload makes, and the only remaining
limit is S3's own — the same 5 GiB figure, by coincidence rather than by
decision.

Create a personal API token with the sole `datasets` scope. A token still acts
as its owner, so it cannot reach a dataset that its owner cannot write. Revoke
it when the import is complete.

Authorisation and confirmation are both audited **per batch** — who, which dataset, how many objects,
how many bytes — — who, which dataset, how many objects, how many bytes, and the sender's
SHA-256 values. Verification itself stays per object: each one is stat-ed
against its declared size, because that is the only check worth making. One entry per file in both places would be eight thousand
lines to move one study, which is a log nobody can read rather than a security
property. Confirmation checks the object exists at the announced size and
records the sender's SHA-256 manifest value.

Confirmation is the sender's statement, not a gate: nothing stops an object
being written and never confirmed. What it buys is a record that says the
object arrived whole, from whom, with which checksum.

## Import from FOR

Copy `tools/noryx_dataset_sync.py` to the sending machine, then run:

```bash
export NORYX_TOKEN='noryx_...'
python3 noryx_dataset_sync.py /data/PREMYOM1000 \
  --url https://datalab.noryxlab.ai \
  --dataset '<dataset UUID>' \
  --prefix PREMYOM1000 \
  --workers 4
```

The script needs Python 3 and `curl`. It writes `.noryx-import-state.json` in
the current directory. Re-run the exact command after a network interruption:
files already confirmed at the same size and modification time are skipped.

It asks for URLs **two hundred at a time**, so moving SELENA costs about twenty
authorisation calls rather than four thousand. And it hashes only what it is
about to send: the first version read the whole study — nine gigabytes,
single-threaded — before a single byte left, and did it again on every resume,
which is exactly when it is least welcome. The SHA-256 is still computed for
every transferred object and still recorded with it.

Use `--dry-run` to list the volume and the first twenty objects before sending
data. Keep workers at four initially; increase only after observing the source
link and the S3 API.

A refused object is named with its reason and the rest of the batch still
goes: one bad name in a directory of four thousand should cost that file, not
the import.

## Which datasets take which route

A presigned URL carries the host the platform's own client talks to. For a
dataset on the internal MinIO profile that is a cluster service, which
resolves inside the cluster and nowhere else — so the direct route is refused
with `direct_upload_unavailable` rather than signed and handed over. The
client reads the code and sends through the API instead.

Datasets on an external endpoint — the HDS buckets on Clever Cloud — take the
direct route.

This was found on the first real import, with 1,957 files failing on `curl`
exit 6 and nothing connecting the failure to its cause. It is the kind of
defect only a transfer from outside the cluster can show.

## Current boundary

The first implementation uses one S3 PUT per object, capped at `5 GiB` per
file. This fits DICOM and imaging trees made of many files. A single object
above this size requires the next capability: signed S3 multipart upload with
part-level resume. Do not split or compress regulated source files merely to
bypass this boundary without recording the transformation.
