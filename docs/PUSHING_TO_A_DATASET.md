# Pushing data into a Noryx dataset

One page, for somebody sending a study from their own machine. The full
reference is [DATASET_BULK_IMPORT.md](DATASET_BULK_IMPORT.md).

## Before you start

You need three things from the platform administrator:

| | |
|---|---|
| **An account** | in the organisation that owns the dataset |
| **`writer` on the target dataset** | `owner` also works; `reader` does not |
| **A personal API token** | scoped to `datasets` only — it reaches no project, job or workspace |

And one thing you already have: the **dataset UUID**, visible in the catalogue
or in the URL of the dataset's page.

A token acts as you. It cannot reach a dataset you could not write yourself,
and it should be revoked when the import is done.

## The short version

```bash
export NORYX_TOKEN='noryx_...'

python3 noryx_dataset_sync.py /data/PREMYOM1000 \
  --url https://datalab.noryxlab.ai \
  --dataset 'f7c1…-the-dataset-uuid' \
  --prefix PREMYOM1000 \
  --workers 4 \
  --dry-run
```

Drop `--dry-run` when the listed volume looks right.

The script is one file, from the public repository:

```bash
curl -O https://raw.githubusercontent.com/Noryxlab/NoryxLab-CE/main/tools/noryx_dataset_sync.py
```

It needs **Python 3 and `curl`**, nothing else — no `pip install`.

**Interrupted? Re-run the exact same command.** It keeps a journal
(`.noryx-import-state.jsonl`) in the working directory and skips what the
platform has already confirmed. Keep that file for the whole import; delete it
only to force a full re-send.

## What actually happens

```
your machine  ──1── Noryx: who are you, may you write here, give me URLs
              ──2── S3 (HDS): the bytes, straight there
              ──3── Noryx: these arrived, this size, this checksum
```

Your files **never pass through Noryx** — when the storage is reachable from
outside. Noryx decides who may write which key and signs short-lived URLs for
exactly those keys; the bytes go from your machine to the storage. No S3
credential is ever given out.

**Some datasets take the other route.** A dataset held on the platform's own
internal object store answers on a cluster address that resolves nowhere
outside it, so no signed URL for it could ever work. Noryx says so instead of
handing you a dead URL, and the script switches by itself:

```
direct upload is not available for this dataset; sending through the API instead
```

Nothing to do about it. The bytes travel through the API, which is slower and
works everywhere. The HDS buckets are on an external endpoint, so a clinical
import takes the direct route.

Two details that matter:

- **URLs last fifteen minutes** and cover one object each. The script asks for
  two hundred at a time and spends them immediately.
- **The destination key is composed by the server** from the dataset's own
  prefix. A path trying to climb out with `../` lands back inside the dataset.

Every authorisation and every confirmation is written to the audit trail, with
your identity, the object count, the bytes and your SHA-256 values.

## Doing it by hand, one file

Useful for a single file or for testing credentials.

```bash
# 1 — ask for a URL
curl --fail-with-body -X POST \
  -H "Authorization: Bearer $NORYX_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"path":"raw/2026-10-08/image.dcm","size":2457600}' \
  https://datalab.noryxlab.ai/api/v1/datasets/$DATASET/upload-url

# 2 — send the bytes to the "url" that came back
curl --fail-with-body -X PUT --upload-file ./image.dcm "$PRESIGNED_URL"

# 3 — tell Noryx it arrived
curl --fail-with-body -X POST \
  -H "Authorization: Bearer $NORYX_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"path":"raw/2026-10-08/image.dcm","size":2457600,"sha256":"<sha256>"}' \
  https://datalab.noryxlab.ai/api/v1/datasets/$DATASET/upload-complete
```

Step 3 is not a gate — the object is already written. It is what records that
it arrived whole, from you, with that checksum.

## Limits, stated plainly

| | |
|---|---|
| One object | **5 GiB maximum.** No multipart yet. |
| Objects per authorisation call | 200 |
| URL lifetime | 15 minutes |
| Thousands of files | **Measured**, 2026-10-08: 1,957 objects, byte-for-byte against the bucket, and a resume of the same tree answered in 0.06 s. Through the API route. |
| The direct route | **Measured**, 2026-10-08: a round trip to the Clever endpoint — presign, PUT straight from the sending machine, confirm — read back and compared. Three objects, not thousands. |
| Tens of GiB in one run | Not measured. Nothing in the design objects; nobody has done it. |
| Hundreds of thousands of files, terabytes | Designed for, not measured. Read the note below. |

**Do not split or compress a regulated source file to get under 5 GiB**
without recording the transformation. A study whose files were silently
reshaped is a study nobody can reproduce.

## At terabyte scale

Nothing in the design breaks — and nobody has run one. Three things are worth
knowing before launching a very large transfer:

- **Your own network is the unknown**, not the platform. The measurements
  above were taken from inside; a transfer out of your site has your egress,
  your proxy and your firewall in the path, and none of that has been tested.
  Send a few hundred files first and look at the rate before committing a
  night to it.

- **Transfer time is the link, not the platform.** The bytes go straight to
  storage, so a terabyte takes as long as a terabyte takes. Run it in `tmux`
  or `screen`.
- **Start with `--workers 4`** and raise it only after watching the source
  link. More workers against a saturated link makes a run slower, not faster.

If a run of that size is planned, say so first: it is worth watching the first
hour together rather than discovering a per-hour limit at 3 a.m.

## When something goes wrong

**`401` / `403`** — the token is expired, revoked, or you do not have `writer`
on this dataset. Ask the administrator to check the dataset's access list.

**`404` on the dataset** — the UUID is wrong, *or* you cannot see it at all.
Noryx answers 404 rather than 403 for a dataset you may not read, deliberately:
whether a given dataset exists is itself information.

**`413` on one object** — it is above 5 GiB. See the limits above.

**`direct upload is not available for this dataset`** — not an error. See
above; the script has already switched.

**`REFUSED <path>`** — that one path was rejected, with its reason, and the
rest of the batch went through. Fix the name and re-run; confirmed files are
skipped.

**`UNVERIFIED <path>`** — the bytes went out but the platform did not find the
object at the announced size. It is **not** written to the journal, so the next
run sends it again. That is the intended behaviour: an object recorded as done
but absent would be skipped forever.

**The run stops with "N object(s) failed"** — re-run the same command. Nothing
already confirmed is sent twice.

## After the import

Tell the platform side. The data is in the bucket but nothing describes it yet:
somebody runs a scan to build the ontology, checks how the paths are read, and
only then can anybody cut an extract and work on it. See
[USING_ONTOLOGIES.md](USING_ONTOLOGIES.md).
