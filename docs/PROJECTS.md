# Projects

## Project catalog cards

The project catalog exposes a concise operational summary for every project:

- owner and short editable description
- latest activity computed from project metadata, apps, jobs, and workspaces
- running application, job, and workspace counts

Project owners and global administrators can update the name and description with `PUT /api/v1/projects/{projectID}`.

## Why a project can come back after being deleted

It used to, under the name `Recovered Project <short id>`, and the reason is
worth knowing because the mechanism is general.

The platform reconciles its records against the cluster: for every workload a
listing returns, it makes sure the project behind it exists, recreating a
missing row under that recovered name. That exists for one case — the project
store lost rows while the workloads survived (see [RECOVERY.md](RECOVERY.md)) —
because a workload whose project is gone belongs to nothing and disappears from
every screen.

Kubernetes deletion is asynchronous. A pod keeps its labels while it drains, so
a workload deleted a second ago still answers a listing. On the EMSE cluster the
nightly platform test suite created a throwaway project at 02:00, exercised it,
deleted everything in the right order — and the project came back at 02:03
because its workspace pod was still terminating when the row went. One phantom a
night, each with a fresh identifier, each to be opened and deleted by hand.

Both listings now skip an object carrying a `deletionTimestamp`. A pod on its way
out is not evidence of anything existing, and reading it as present also put a
workspace back on the screen it had just left.

**If you see one anyway**, it means a labelled object outlived its project for
longer than a drain: look for a pod stuck terminating on a finalizer, or a job
nobody collected, with
`kubectl get pods,jobs -A -l noryx.io/project-id=<id>`. The project row is the
symptom; the object is the thing to remove. A phantom holds nothing — no
workspace, no app, no job, no member — which is how it is told apart from a real
project whose row was genuinely lost.
