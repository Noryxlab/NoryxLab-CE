# RBAC Model (CE and EE baseline)

This document defines the access-control baseline used from day one.

## CE (Community Edition)

CE keeps the model intentionally minimal:

- `admin`
- `user`

### CE permissions

- `admin`
  - full platform access
  - sees all projects (cross-project visibility)
  - can access admin modules (`users`, `modules`, `workloads`)
  - can invite collaborators and manage project member roles
  - can create/update/delete all catalog objects (datasets, repositories, secrets)
- `user`
  - can access non-admin product modules
  - can manage own objects
  - each created project belongs to the creator (`project admin` on that project)
  - can invite other users only on projects where user is `project admin`
  - can create and use project-scoped workloads (workspaces, jobs, apps, dashboards)
  - cannot access platform admin modules

### Dataset permissions

Dataset permissions are independent from project roles:

- `owner`: dataset paternity, read/write, delete, project assignment, and ACL management
- `writer`: read and object upload/update
- `reader`: read only

Dataset ACLs are managed by the dataset owner or a global admin. Attaching a
dataset to a project remains a separate operation because it exposes the dataset
to project workloads. Regulated HDS policies are an Enterprise Edition concern.

### Project ownership

Every project has one owner:

- a user, by default the user who created the project
- an organization, after an ownership transfer

The project owner has project-administrator permissions. When an organization
owns a project, every current member of that Keycloak organization can see and
administer the project. Individual project memberships remain valid in addition
to ownership.

Only the current owner or a global administrator can transfer project
ownership. A non-admin user can only transfer ownership to an organization they
belong to.

## EE (Enterprise Edition)

EE extends CE with a custom role matrix:

- built-in roles: `admin`, `user`
- custom roles: defined by administrators

EE can also require every authenticated user to belong to an organization.
Keycloak owns organization membership; NoryxLab owns authorization decisions.
The delivered organization scope covers mandatory membership, administrative
membership management, organization-owned projects and organization-owned
datasets.

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
