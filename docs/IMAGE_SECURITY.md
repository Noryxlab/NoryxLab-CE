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

## Rebuilding: nightly, at 02:40

`noryx-environment-rebuild` runs kaniko over the three system Dockerfiles from
the public Community repository and pushes over the tags the installation
runs. Without a cache, so the base image is fetched fresh and its security
fixes come with it.

It was written on 2026-09-18, and what was there before was a sentence. The
paragraph above had said "rebuild weekly" since the platform was written, no
scheduler existed on either installation, and the three system images were 46,
46 and 28 days old. An intention with no scheduler is not a policy - the same
shape as a vulnerability column that could never fill.

**Two dependencies, both silent when they are missing.**

*The digest.* Workspace pods run with `imagePullPolicy: IfNotPresent`, so
overwriting a tag reaches no node that already holds it. The platform resolves
the tag to a digest before it starts a pod, which turns a rebuilt image into a
new reference and gets it pulled - and that resolution goes through Harbor's
API with the platform's robot account. On EMSE that account received 403, so
pods ran bare tags, and a nightly rebuild would have been work nobody
received. The robot needs **read on repositories**, the same grant the scan
column needs.

*The profile.* A person's extensions live in their profile volume, not in the
image. See below.

## Extensions, and how a rebuilt image reaches somebody

Extensions are installed into the image at build time and copied into each
person's profile volume at first launch. The copy used to happen only into an
empty directory - which meant once per volume, ever. Somebody who opened a
workspace in August kept August's extensions no matter how often the image was
rebuilt.

The image now carries `/opt/noryx-vscode/IMAGE_BUILD`, stamped at build time,
and the profile remembers the last stamp it was given. When they differ the
bootstrap copies the extensions again, **before the editor starts**. Extension
directories are merged, so anything somebody installed themselves survives;
`extensions.json` is deleted rather than overwritten, because it is VS Code's
manifest of that directory and replacing it with the image's would erase every
extension the person had added. It is rebuilt from what is on disk.

**Auto-update from the Microsoft marketplace is switched off** in the image.
It was on, and it worked: on 2026-09-18 a workspace upgraded `charliermarsh.ruff`
from 2026.80.0 to 2026.82.0 during launch, from `marketplace.visualstudio.com`.
Three reasons to stop doing it: it costs launch time in the worst possible
place, it makes the same pinned image produce different tooling depending on
the day, and it means every workspace on a platform holding regulated data
calls Microsoft at start-up. The nightly rebuild produces the same result,
once, off-hours, identically for everyone.

## Seeing them: scan on push

Harbor scans every artifact it receives with Trivy, and reports counts by
severity. The environment catalogue shows those counts on each environment's
row: `3 critical`, `14 high`, or nothing at all.

**Nothing at all means the registry did not say**, not that the image is
clean. A registry with scanning switched off reports no findings, and a screen
that rendered that as a green "0" would be worse than a screen that says
nothing.

Saying nothing is not enough either, which took a year to notice. The column
distinguishes three answers:

| Shown | Means |
|---|---|
| `3 critical`, `14 high`, or **None** | The registry scanned this image and this is what it found |
| **Not scanned** | The registry holds the image and has not scanned it |
| **Unknown** | The platform could not ask, and the reason is in the tooltip |

The third is the one that matters, and it was invisible. On the EMSE
installation every environment showed a dash, which reads as reassurance; the
cause was that the platform's robot account could push and pull images but
received **HTTP 403** on Harbor's scan API. A missing permission and a clean
image rendered identically. The refusal is now logged on every lookup as well
as shown, because a permission an administrator has to grant will not be
noticed in a table cell.

To switch scanning on, in Harbor: *Administration → Interrogation Services*
(the Trivy scanner is bundled with Harbor 2.x), then per project
*Configuration → Automatically scan images on push*. The robot account the
platform uses also needs **read on repositories and on scan reports** in each
project it is expected to report on - pushing and pulling is not enough.

## Where a project's builds go

Derived build destinations are pushed to a registry project of their own, a
sibling of the platform's: `noryx-environments` holds the images the platform
starts workspaces from, `noryx-environments-builds` holds what projects build.
`NORYX_BUILD_REGISTRY_PROJECT` names a different one outright.

**This project has to exist in the registry before a build can push to it**,
and the platform's robot account needs push on it. Harbor does not create it
on demand and a robot account may not create it either, so this is a one-time
step for whoever administers the registry.

Why they are separated at all: until 2026-09-18 both lived in the same
registry project, distinguished only by a prefix in the repository name. On
2026-09-07 a build reached `noryx-vscode` - the repository the platform starts
VS Code workspaces from. Catalogue entries are keyed on the repository, so the
build merged with the system environment, and the list offered the system name
while carrying the build's image. Every launch from that entry was refused,
for an image nobody could see had been substituted. A guard now refuses that
destination by name; the separate project removes the adjacency that made it
reachable, and lets the registry's own access control do the rest.

## What a revision records

Each revision carries the commit its ref pointed at when the build was
submitted, and the digest of the image it produced. Both are shown in the
revisions table, shortened, with the full value in the tooltip.

Neither was recorded before 2026-09-18, and the pair is what makes a build an
account of itself. A ref is a branch: "built from `main`" names a different
commit every week, so a record holding only the ref cannot be replayed and
cannot say which source produced a running image. A tag can be overwritten in
the registry; a digest cannot.

The commit is resolved when the build is submitted, by asking the remote which
commit the ref points at - the request `git ls-remote` makes, with no clone and
no credential. It is therefore **empty for a repository the platform cannot
read anonymously**, and for `git@`/`ssh://` remotes. The digest is read from
the registry once the build has succeeded. Empty means unknown and is displayed
as unknown: neither value is ever inferred.

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
