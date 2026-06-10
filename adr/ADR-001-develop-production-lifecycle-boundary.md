# ADR-001: Separate development lifecycle from production operations

## Status

Accepted

## Context

Noryx supports project-scoped creation and execution of workspaces, jobs, apps,
dashboards, and API services.

Mixing creation forms, lifecycle management, and production operations in the
same navigation makes it unclear whether a service is still being developed or
is considered published. It also prevents Noryx from introducing explicit
production controls without making project screens more complex.

Noryx must provide a clear path from development to production while preserving
project ownership, visitor RBAC, operational visibility, and future governance
controls.

## Decision

Noryx separates the product into two lifecycle areas:

- `Develop` owns project-scoped creation, testing, publication, and lifecycle
  management.
- `Deploy > Production` provides a transverse, read-oriented operational view of
  services published from accessible projects.

The production inventory exposes the service type, project, runtime status,
visitor RBAC, URL, and publication date. It does not contain creation forms or
destructive lifecycle actions.

An app, dashboard, or API service is a production resource once it has been
published through its project-scoped Develop workflow. Running a workspace or a
job does not publish it to production.

Future production lifecycle controls must extend this boundary with explicit
service revisions and promotion operations rather than moving creation forms
into Deploy. These controls include:

- immutable published revisions
- promotion and approval workflows
- staged environments
- rollback
- production health and incident visibility
- production-specific audit events and authorization policies

## Consequences

Benefits:

- users can distinguish development resources from production services
- production operators get a transverse platform view
- project teams retain control of service creation and publication
- future governance and promotion controls have a stable architectural boundary
- Noryx explicitly supports production workloads rather than only development
  workspaces

Trade-offs:

- publication semantics must be made explicit for every production service type
- production state and revision history require durable backend models
- authorization must distinguish project lifecycle actions from production
  operations
- the current direct publication flow will need migration toward explicit
  revisions and promotions

## Edition boundary

The Develop/Deploy separation and production inventory are CE capabilities.

Advanced production governance, such as multi-stage approvals, policy-driven
promotion, regulated controls, and advanced operational audit, may be provided
as additive EE capabilities through CE extension points.
