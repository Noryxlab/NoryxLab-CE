# CE Extension Points for EE

NoryxLab-CE keeps enterprise hooks explicit so NoryxLab-EE can extend behavior without forking CE handlers.

## Package

- `backend/internal/edition/hooks.go`

## Contracts

1. `RBACProvider`
- override global admin resolution
- override access to admin modules (`users`, `modules`, `workloads`)
- override project-action authorization with `CanAccessProjectAction(...)`
  - action IDs used by CE core:
    - `project.read`
    - `project.launch`
    - `project.build`
    - `project.manage_members`

2. `FeatureGate`
- central feature flags by name (default CE = disabled)

3. `AuditSink`
- structured event sink for sensitive actions (default CE = no-op)

## CE default behavior

- CE runs with `DefaultHooks()`.
- No enterprise feature is enabled by default.
- Existing CE behavior stays unchanged.

## EE integration strategy

- EE provides concrete implementations for RBAC/Feature/Audit.
- EE wiring is injected at handler initialization.
- Authorization decisions remain backend-side.
