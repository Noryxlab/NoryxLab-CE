# Replaying a run

A finished job can be re-executed from what that run recorded. The button is in
the row menu on the Jobs page, and the record it starts from is shown above the
logs when a job is selected.

This page is the replay. What a run writes down about the data it read is in
[Job history and results](JOB_HISTORY.md); the decision behind both is
`ADR-056`.

## What a run records

At the moment it starts, a job writes down:

| What | Recorded |
| --- | --- |
| Image | the reference it was given **and the digest it resolved to** |
| Datasets | identifier, name, bucket, prefix, and whether it could write |
| Extracts | by name — a frozen selection, so naming it names the files |
| Repositories | name, URL, ref, and the commit the clone landed on |

The digest is the part that makes a replay a replay. A tag designates whatever
it designates today: the nightly rebuild moves `noryx-python:0.1.5` to a new
digest when an upstream extension changes, which is correct for a shelf of
maintained images and fatal to the sentence "we re-ran the calculation".

The commit is resolved by asking the forge, carrying the repository's stored
credential. A forge that does not answer costs that run its commit, recorded as
unknown — the screen then says the code of that run cannot be identified,
rather than showing a blank.

## What a replay does

- Launches **the digest** that actually ran, not the tag.
- Keeps the command, the arguments and the machine size.
- Creates **a new job**. The original and its record are untouched: a
  provenance record a replay overwrote would no longer be a record. The two
  are linked by `rerunOf`.
- Runs **with your rights**, not those of the person who launched the first
  one. This is the one thing a replay deliberately does not reproduce: a replay
  that handed back the first launcher's data would be a privilege escalation
  dressed as a convenience.

The replay is offered only once a run has finished. Replaying a job that is
still going reproduces nothing; it doubles the bill.

## What it tells you afterwards

A replay lists how it differs from what it replays, in sentences:

```text
dataset "cohorte-2026" was mounted by the original run and is not mounted now
repository "qwen-eval" moved from 3f2a1c04 to 9b7e5512
image: no digest was recorded for the original run, so its tag is used
  and may have moved
```

Named rather than counted on purpose. "Forty-one differences" sends somebody
looking; the line above tells them what happened. The list is sorted, so two
readings of the same pair of runs produce the same text.

A replay with no differences says **"faithful replay — same image digest, same
datasets, same commits"**. That sentence is shown rather than an empty list: an
absent list and an empty one read the same, and only one of them is true. A run
that is not a replay has nothing to compare and is not awarded a fidelity that
would mean nothing.

## What it does not do

**Nothing is backfilled.** Runs from before 2026-11-22 have no digest and no
commits. They can still be replayed, and the replay says it is starting from a
tag that may have moved.

**It does not reproduce the data.** The datasets are named, not frozen. A
dataset rewritten since the first run gives a replay that reads different bytes
under the same name. [Dataset snapshots](DATASET_SNAPSHOTS.md) are what detect
that, and the two capabilities were decided together for this reason.

**It does not record what the run produced.** Lineage stops at the input. "Which
model version, fed by which data, produced this file" is answered on the input
side only; the output side is deliberate work still to do.
