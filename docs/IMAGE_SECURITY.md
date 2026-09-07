# Keeping environment images current

An environment is an image, and an image ages: the base it was built from
receives security fixes, and the copy in your registry does not. This is how
the platform handles that, and what it deliberately does not do.

## Where the vulnerabilities are

Most of them are in the **base image**, not in what a project adds on top. A
`pip install pandas` brings a handful of Python packages; `python:3.12-slim`
brings a Debian userland. So rebuilding an environment without moving its base
fixes almost nothing, and moving the base fixes most of it.

That is why the answer is a **rebuild from a moving base tag**, and not a
nightly `apt-get upgrade` inside a frozen image.

## Seeing them: scan on push

Harbor scans every artifact it receives with Trivy, and reports counts by
severity. The environment catalogue shows those counts on each environment's
row: `3 critical`, `14 high`, or nothing at all.

**Nothing at all means the registry did not say**, not that the image is
clean. A registry with scanning switched off reports no findings, and a screen
that rendered that as a green "0" would be worse than a screen that says
nothing. So the column shows a dash until a scan exists.

To switch it on, in Harbor: *Administration → Interrogation Services* (the
Trivy scanner is bundled with Harbor 2.x), then per project *Configuration →
Automatically scan images on push*.

## Not blocking

Harbor can refuse to serve an image whose severity exceeds a threshold. The
platform does not use it, on purpose: a workspace that will not start because
a scanner found a high-severity CVE in a library nothing calls is how people
learn to work around the platform. The finding is shown to whoever can act on
it, and the decision stays theirs.

## Keeping the registry from filling up

Every rebuild pushes a new revision - `:r1`, `:r2`, `:r3` - and moves
`:latest`. Revisions of one environment share every layer they have in common,
so ten revisions of a 1 GB image cost 1 GB plus the deltas, not 10 GB.

What does fill a registry is never cleaning up. Two Harbor settings, per
project:

- **Retention**: keep the last 5 pushed artifacts, **and** anything pulled in
  the last 30 days, **and** anything tagged `latest`. The second rule matters
  because the platform pins images by digest: deleting a revision a running
  workload still references means that workload cannot restart.
- **Garbage collection**: weekly, Sunday. Deleting a tag frees nothing until
  GC runs, which is why registry cleanup so often looks like it does nothing.

## What is still manual

The weekly rebuild is not automated yet. Rebuilding an environment is a click,
and nothing schedules it. When it is automated it will be opt-in per
environment, weekly rather than nightly - a rebuild changes the image digest,
and a platform that changes every digest every night has given up on
reproducibility to chase a `apt-get upgrade` that may fix nothing.
