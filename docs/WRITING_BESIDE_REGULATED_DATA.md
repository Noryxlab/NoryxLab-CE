# Why a dataset is read-only beside regulated data

One page for the person who just hit this, and one section at the end for the
administrator who decides what to do about it.

## What you saw

A workspace, a job or an application started, your regulated dataset is there
and readable, and a dataset you own and normally write to refuses the write.
The launch said so, in the same place it tells you what was withheld for want
of a right:

> **Read-only in this session** — *Mounted read-only, although you may write
> them: this session also mounts regulated data, which may not be copied out of
> its perimeter. Write your results to a regulated dataset.*

A job says the same thing and adds the consequence: *a job that writes to this
dataset will fail.* The launch also records which mounts it demoted, so the
run's provenance carries it afterwards.

Nothing is broken, and nothing about your rights changed. You still hold
`writer` on that dataset, and you can write to it from a workspace in a
project that holds no regulated data.

## Why

A workload that mounts a regulated dataset is in the perimeter that dataset is
supposed to stay inside. The destination next to it — your own dataset, with
no classification — is **outside** it: anybody who owns it may download it
whole, legitimately, because no rule refuses a dataset nobody classified.

So a single `cp` inside a session moves health data out of the perimeter and
leaves it somewhere it can be taken home. And it leaves no trace: writes
through a dataset mount go from the node straight to the object store, never
through the API, which is the only place the platform records writes. This is
not theoretical — it is how 17 GB of a research cohort came to sit in a
personal dataset on a production installation, and it was found by accident.

Rather than let the copy happen and hope to notice, the platform removes the
destination: while a regulated dataset is mounted, the mounts that are not
themselves regulated are mounted read-only.

**Writing from one regulated dataset into another stays allowed.** That is the
important exception and the normal case: a destination carrying the same
classification is inside the same perimeter, closed the same way, with its
bulk download refused the same way.

## What to do

In the order to try them.

**1. Write into a regulated destination.** The usual shape is a second
regulated dataset beside the data, for outputs: `my-cohort` holds the data and
`my-cohort.workspace` takes your results. You create it yourself from the
catalogue if your role carries `register-regulated-dataset`, pointing it at a
bucket your own credentials already reach. That function is **not** on the
supplied `user` role — an installation that expects researchers to create
their own regulated outputs has to add it, and one that keeps it with a named
data steward will have you ask them instead. A prefix of the cohort's own bucket is a perfectly good target; a
dataset is a bucket plus a prefix.

**2. Write into the project's own space.** `/mnt` is the project volume,
persistent at project scope and shared by the project's workloads. It is not a
dataset, so the rule does not touch it, and it outlives any one workspace. It is the right place for code, notebooks, intermediate files and
anything you are still working on. It is **not** the place for a regulated
output you mean to keep: a project volume is platform storage, not the
certified storage your data has to live on, and your installation's data
protection officer will tell you where that line falls.

**3. Split the work.** If what you are producing has nothing to do with the
regulated data, run it from a project that does not attach any. Two projects
is often clearer than one project with two regimes.

**4. Ask for the right.** If your work genuinely needs to write beside
regulated data — and some does — an administrator can grant your role the
function below. Expect to be told what it trades.

## For the administrator: the right, and what it costs

The capability is **`write-beside-regulated-data`**, on the role matrix in
*Administration → roles*. Nobody holds it by default, including the supplied
`user` role.

A role carrying it keeps its unclassified mounts writable while the same
workload reads regulated data. The rule above simply does not apply to them.
It lifts that and nothing else: a mount that is read-only because the person
holds `Read` on the data stays read-only.

**What you are trading.** Confinement for detection after the fact. Without
the right, the copy cannot happen. With it, the copy can happen and the
platform looks for it every night — which means a copy can exist for up to a
day before anybody knows. If your installation cannot accept that, do not
grant the function. If it can, grant it to named people rather than to the
`user` row, so the list of those who hold it is short and readable.

Granting it is audited, like every change to the matrix.

## The nightly crossing

This is what replaces the rule for those who hold the right, and it runs on
every installation whether or not anybody holds it.

Each night, for every pair made of **one regulated dataset and one
unclassified dataset attached to the same project** — the only situation a
single workload could have mounted both — the platform lists each side and
reports the objects it recognises on both, with two confidences:

| | what it means |
|---|---|
| **Same fingerprint** | Identical ETags. For an object uploaded in one part the ETag is the content's MD5, so this is an assertion, not a suspicion. |
| **Same size and name** | Identical byte size and identical file name, different ETags. That is what a copy of a large file looks like — it is uploaded in parts, and each part changes the ETag. Nothing under 1 MiB is reported this way: small objects are already covered by their fingerprint, and below that limit "same name, same size" only ever names empty `README`s. |

The alert goes to the administrators, names both paths of every object it
recognised, and says plainly that it knows *what* and not *who* — because
nothing records who wrote through a mount, which is the entire reason this
check exists.

It reads listings, never bytes, and only the pairs a project actually mixes.
On a typical installation that is one project and a handful of pairs.

**Run it now instead of waiting:** *Administration → Data usage map →
Fingerprint crossing → Cross now*. The report comes back on screen. An
administrator who has just granted the right uses this rather than waiting a
day.

### What the crossing cannot see

Stated here because "nothing found" must not be read as "nothing left":

- **A derived result carries the fingerprint of nothing.** An aggregate, a
  trained model, a de-identified export: all of them are regulated data in a
  new shape, and none of them matches an object in the cohort.
- **Data that leaves by the network, or through your own session,** lands in
  no bucket, so no listing holds it. The network is closed per project by the
  egress rules, and the download from an editor is bounded by who holds `Read`
  — not by this check. See
  [how data leaves a workspace](HOW_DATA_LEAVES_A_WORKSPACE.md).
- **A copy laid into a dataset not attached beside the regulated one** is
  outside the compared perimeter. Reaching it takes two steps rather than one,
  and comparing every bucket against every other would cost a full listing of
  the platform every night.

And no detection prevents. A copy found in the morning is a copy that exists.
That is the honest shape of the trade, and the reason the rule remains the
default.

## Reference

The decisions, with the reasoning and the alternatives that were declined:
`ADR-055` (the rule) and `ADR-060` (the right and the crossing), in the Noryx
project repository. The capability model is [RBAC_MODEL.md](RBAC_MODEL.md);
the full list of ways data can leave is
[HOW_DATA_LEAVES_A_WORKSPACE.md](HOW_DATA_LEAVES_A_WORKSPACE.md).
