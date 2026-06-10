# Production

The architectural decision is documented in NoryxProject:
[`ADR-022: Separate development lifecycle from production operations`](https://github.com/Noryxlab/NoryxProject/blob/main/adr/ADR-022-development-and-production-lifecycle-boundary.md).

Noryx separates project-scoped publication from production operations:

- `Develop` contains project-scoped creation and lifecycle management for apps,
  dashboards, and future API services.
- `Production` is a transverse operational inventory of published
  services across every project accessible to the current user.

The production inventory exposes service type, project, runtime status, visitor
RBAC, public URL, and publication date. It intentionally does not expose launch
forms or destructive actions.

A future promotion workflow can extend this boundary with revisions, approvals,
rollbacks, and staged environments without mixing those concerns with project
creation screens.
