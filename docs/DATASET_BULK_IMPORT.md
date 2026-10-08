# Bulk dataset import

Use this path for a clinical import such as PREMYOM-1000 or SELENA: thousands
of files, tens of GiB, and a sender outside the platform network.

## Security model

Noryx authenticates the sender and checks their `writer` access to exactly one
dataset. It returns a fifteen-minute S3 URL for exactly one object. The file
bytes then travel directly from the sender to the configured object store;
Noryx never exposes the S3 access key or proxies the health data.

Create a personal API token with the sole `datasets` scope. A token still acts
as its owner, so it cannot reach a dataset that its owner cannot write. Revoke
it when the import is complete.

Every authorization and every confirmed object upload is written to the EE
audit trail. Confirmation checks the object exists at the announced size and
records the sender's SHA-256 manifest value.

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
only files already confirmed with the same size and SHA-256 are skipped.

Use `--dry-run` to list the transfer volume before sending data. Keep workers
at four initially; increase only after observing the source link and S3 API.

## Current boundary

The first implementation uses one S3 PUT per object, capped at `5 GiB` per
file. This fits DICOM and imaging trees made of many files. A single object
above this size requires the next capability: signed S3 multipart upload with
part-level resume. Do not split or compress regulated source files merely to
bypass this boundary without recording the transformation.
