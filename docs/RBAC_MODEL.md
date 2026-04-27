# RBAC Model (CE and EE baseline)

This document defines the access-control baseline used from day one.

## CE (Community Edition)

CE keeps the model intentionally minimal:

- `admin`
- `user`

### CE permissions

- `admin`
  - full platform access
  - can access admin modules (`users`, `modules`, `workloads`)
  - can manage project member roles
  - can create/update/delete all catalog objects (datasets, repositories, secrets)
- `user`
  - can access non-admin product modules
  - can manage own objects
  - can create and use project-scoped workloads (workspaces now, jobs/apps/apis later)
  - cannot access platform admin modules

## EE (Enterprise Edition)

EE extends CE with a custom role matrix:

- built-in roles: `admin`, `user`
- custom roles: defined by administrators

### EE matrix model

Each role is configured using:

- role name
- object scope
- allowed actions

Object scope examples:

- projects
- workspaces
- jobs
- apps
- apis
- datasets
- repositories
- secrets
- environments
- ops modules

Action examples:

- `none`
- `read`
- `write`
- `admin`

EE UI target behavior:

- admin can create role entries with a matrix form
- each row = role + object
- each value selected from a dropdown of allowed actions
- effective permissions are evaluated server-side

## Notes

- CE remains simple by design and avoids role proliferation.
- EE keeps CE compatibility while adding enterprise-grade delegation.
- Backend authorization must stay the source of truth; UI only reflects capabilities.
