# Flows

A flow is a graph of jobs: *mesh this, then simulate it, and render both while
you are at it*. The page is in the project, beside Jobs. The decision is
`ADR-059`.

A flow is **not** a workflow. Workflows (see [Agents](AGENTS.md)) are what an
agent executes — a sequence of reasoning and approval steps, with no graph.
Flows are compute, and the shape of compute is a graph because the work is one.

## A step is an ordinary job

Every step goes down the same launch path as any job you start by hand, so it
gets the same mounts, the same egress profile, the same rights resolution, the
same read-only demotion under the co-mount rule, and the same provenance
record. A pipeline whose steps ran through a path of their own would drift away
from ordinary runs, and the day that showed would be the day a result could not
be explained.

In practice that means a step's pod prepares the environment first — the
project's `requirements.txt` into the venv, the attached repositories cloned —
before running its command, exactly as a hand-launched job does. On a small
graph the preparation is most of the wall-clock time.

## Declaring one

A step carries:

| Field | What it is |
| --- | --- |
| Key | stable across runs; what lets a resume say "this one already succeeded" |
| Label | what a person reads |
| Environment | the image, chosen from the catalogue |
| Machine size | as for a job |
| Command | one line, split like a shell would |
| After | the steps it waits for |

Dependencies can only name steps written above the current one, which makes a
cycle impossible to type. The server validates the graph anyway, because it is
the only thing that may — and it validates when the flow is **saved**, not
when it runs: a cycle discovered halfway through leaves pods behind and
somebody waiting, while its author is no longer at the keyboard.

**At most N at once** bounds the fan-out, 3 by default. A graph that opens into
fifty branches must not ask the cluster for fifty pods because its author drew
fifty boxes.

Declaring and running a flow needs the right to launch a job and nothing more.
A flow is only jobs chained together; a separate right would protect nothing
while blocking the useful gesture.

## Running one, and reading what happened

A scheduler advances every active run: it collects what finished, propagates
failures, launches what has become runnable, and settles the run. The state
lives in the database, not in a process, so a run survives a redeployment
instead of dying halfway through with pods still going.

Two views of a run, which do not say the same thing:

- **Graph** — the structure, in columns by longest path. Two steps in the same
  column can run together; that is what the view exists to show. No arrows are
  drawn between columns: a wrong arrow is worse than none, so each step carries
  the list of what it waits on, which is the same information and always exact.
- **Timeline** — what actually happened, one bar per step on the run's own
  clock. Two bars starting at the same instant are the proof of the fan-out
  that the graph only showed as a possibility.

A step's status is one of:

| Status | Meaning |
| --- | --- |
| `pending` | not started; the screen says what it is waiting on |
| `running` | its job is going |
| `succeeded` | done |
| `failed` | its job failed |
| `skipped` | it will never run, because something it depends on failed |
| `reused` | a resume did not re-run it, because it had already succeeded |

`skipped` is its own word and not a failure. On a graph of twelve steps a
person has to be able to see which one broke among the ones that merely never
started, so the screen names the failed steps above the graph.

## A failure stops only what depends on it

Branches that do not need the failed step keep going. Failing the whole graph at
the first failure would throw away work that was fine — and on a mesh that
took forty minutes, that is the difference between a usable tool and one
people stop using.

## Resuming

*Resume* starts a new run that carries over the steps which already succeeded
instead of re-running them. Those steps are recorded as `reused`, which is
**not** `succeeded`: a run must never claim to have executed something it did
not. The original job identifier travels with the step, so the provenance of a
reused step still points at the run that actually produced it.

Resume is offered on a run that finished and did not succeed. Resuming a
successful run would reuse everything; resuming one that is still going would
launch the same jobs twice.

## What flows do not do

- **No data is passed between steps.** A step is a job; one step hands work to
  the next through the project volume or a dataset, as two hand-launched jobs
  would. Nothing checks that a step produced what the next one expects.
- **No cancellation.** Stopping a run means deleting its jobs.
- **No schedule.** A flow is started by hand or through the API. Scheduled jobs
  exist separately and do not know about graphs.
- **No retry.** A failed step fails; the answer is a resume, which re-runs it
  as a new attempt.
- **A past run is not rewritten when the flow is.** If a step has been removed
  from the graph since, the run that executed it says so rather than hiding it.
