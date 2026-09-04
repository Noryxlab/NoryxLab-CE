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

### Project membership roles

These are the roles an administrator assigns on the project members screen, and
they are what `viewer`, `editor` and `admin` mean everywhere in the API. They
were missing from this document for as long as the feature existed, so the one
place a reader could learn what a contributor may do was the interface itself.

| Role | May |
|---|---|
| `viewer` | read the project and the results of its work |
| `editor` | everything a viewer may, plus launch workspaces, jobs and apps, run builds, and attach or detach catalogue objects |
| `admin` | everything an editor may, plus manage members and organization grants |

Four actions are decided against these roles, and they are the vocabulary the
Enterprise role matrix extends:

| Action | `viewer` | `editor` | `admin` |
|---|---|---|---|
| `project.read` | yes | yes | yes |
| `project.launch` | no | yes | yes |
| `project.build` | no | yes | yes |
| `project.manage_members` | no | no | yes |

A global administrator, and the owner of the project, pass every check.

### Roles granted to an organization

A role may be granted to a Keycloak organization instead of to a person. Every
member of that organization then holds it, and membership changes take effect
without touching the project.

Grants **add up**: the strongest of a person's own role and any role reaching
them through an organization applies. Removing someone from an organization
never takes away access they were given personally, and a personal `viewer`
role never caps an organization's `editor` grant — either behaviour would give
an administrator's action an effect they did not ask for and cannot see.

If the identity provider cannot be reached, organization grants resolve to
nothing while personal roles still apply. A directory outage must not hand out
access, and must not remove the access somebody already had.

This is distinct from **owning** a project, below: an owning organization
administers it outright, while a grant gives its members one specific role.

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

## Managing accounts

An administrator creates accounts and resets passwords from the administration
screen. Keycloak remains the source of truth for identity — nothing is stored
here; the platform decides only who may ask.

**The platform chooses the password, not the administrator.** Someone inventing
one under time pressure picks weak and reused passwords. Twenty characters from
an alphabet without `O/0` or `l/1/I`, which is beyond guessing and still safe to
read aloud — a temporary password is dictated far more often than anyone admits.

It is set `temporary`, so Keycloak adds the `UPDATE_PASSWORD` required action
and the user must choose their own at first sign-in. The window in which the
administrator's copy works therefore ends there. That required action must be
enabled on the realm; if it is not, Keycloak ignores the flag silently and the
administrator hands out permanent passwords believing otherwise.

Creating an account asks for an organization. Where membership is mandatory, an
account created without one signs in and can do nothing — the shape that left
the nightly backup refused for three nights. It is required up front rather than
discovered afterwards.

Creating the account and setting its password are separate calls. A failure
between them leaves an account nobody can sign into, rather than one whose
password nobody recorded, and the error says so with the identifier.

Not yet available: an emailed invitation or reset link. Keycloak supports it
(`execute-actions-email`) and it needs SMTP on the realm; without SMTP the call
succeeds while no message is sent, which is the kind of silence this platform is
trying to remove.

## Personal API tokens

A user calling the API outside a browser — a CI job, a notebook, a script —
presents a personal token as a bearer credential:

    Authorization: Bearer noryx_<id>_<secret>

The token **acts as its owner and holds no rights of its own**, so every check
that would run for a browser session runs unchanged. That is what makes it safe
to issue: a leak costs one account, not the platform. It is therefore a
different thing from the platform service token, which identifies a component
of the platform itself and does carry administrator rights.

Only the hash of the secret is stored, so a copy of the database is not a set of
working credentials, and the secret is shown exactly once — at creation.
Revocation is a stamp rather than a deletion, so an auditor can say when access
ended. A token names itself, because revoking the right one should not require
guessing.

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
