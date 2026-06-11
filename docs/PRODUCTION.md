# Production

The architectural decision is documented in NoryxProject:
[`ADR-022: Separate development lifecycle from production operations`](https://github.com/Noryxlab/NoryxProject/blob/main/adr/ADR-022-development-and-production-lifecycle-boundary.md).

Noryx separates project-scoped publication from production operations:

- `Develop` contains project-scoped creation and lifecycle management for apps,
  dashboards, and future API services.
- `Production` is a transverse operational inventory of published
  services across every project accessible to the current user.

The production inventory exposes project, active revision, runtime status,
visitor RBAC, public URL, publication date, and immutable revision history. It
intentionally does not expose launch forms.

For applications:

- `Develop > Apps` explicitly publishes the active runtime as a new immutable
  revision;
- publication captures both Noryx application metadata and the Kubernetes pod
  manifest;
- `Production` only lists explicitly published applications;
- rollback restores the selected runtime manifest and marks its revision active;
- publication and rollback emit dedicated audit events.

## Operational prerequisite

Before an application can be promoted to Production, its development lifecycle
must be operable from the project:

- runtime health is derived from the Kubernetes pod and service readiness;
- logs are accessible to project members allowed to launch workloads;
- restart gracefully recreates an active pod from its current specification;
- stop removes the runtime pod while retaining the application record;
- delete permanently removes the application runtime and record.

These operations remain in `Develop`. Production rollback requires explicit
confirmation and never creates or edits an application.

A future promotion workflow can extend this boundary with revisions, approvals,
rollbacks, and staged environments without mixing those concerns with project
creation screens.
