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

### Owning is not reading

Two questions were one. Belonging to an organization granted the **owner**
role on everything it owns - read, write, delete, and the right to hand out
access - so on a platform where eight HDS buckets are organization-owned, an
account created and dropped into an organization held, on its first day, the
power to delete a health dataset it had never been introduced to. An account
made for a test on 2026-10-23 is what showed it.

The two questions are now answered separately, for datasets, ontologies and
extracts alike:

- **Owning** is administering: transfer, rename, delete, and decide who gets
  access. It reaches the owning user, the members of the owning team, and -
  for an organization - the people and teams that organization **named** as
  its administrators. Belonging to it confers nothing: the directory says
  where somebody works, which is a fact and not a permission. An organization
  that has named nobody is administered only by platform administrators,
  which is the direction that fails closed.
- **The right over the data** - read, write, mount, fill an extract - comes
  from being the owning user, belonging to the owning team, or holding an
  explicit grant. Ownership by an organization grants nothing here; an
  explicit grant *to* an organization does, because somebody decided it.

Which gives three deliberate levels and one inheritance that no longer
exists. Grant to an organization and everybody who works there reads it,
teams included - that is a choice, made on a screen, revocable in one click.
Grant to a team and belonging to its organization is not enough to see it.
Own something, and you decide those two questions without being able to open
a file.

So a person who runs an organization may hand its bucket to a team without
being able to open one file in it, and a researcher granted that bucket may
read every file without being able to give it away. Both see the object in
their catalogue, because deciding who may read something requires seeing that
it exists - and the catalogue shows what each may do with it.

A team is the unit somebody is deliberately added to, which is why it carries
the data right; membership of an organization says where a person works.

**Naming an organization's administrators** is done under Administration,
People, on the organization itself, and it is deliberately the section below
its members: the list above says where people work, the one below says what
they may do. `organization_admins` holds it, a person or a team, and
`GET/PUT/DELETE /api/v1/admin/organizations/{id}/administrators` is its API.

Project roles are untouched: an organization grant on a project still reaches
its members. What changed is the right over the data, which is the thing this
document separates from ownership.

Four objects answer this way - datasets, ontologies, extracts and data source
connectors - and the four screens are the same screen. Each carries an owner
(mandatory, transferable to a person, a team or an organization) and a list of
grants, reader or writer, shown by name rather than by alias or id.

What follows from the split, stated once because each of these was a separate
bug before 2026-10-23:

| Act | Which right it needs |
|---|---|
| Transfer, rename, delete, grant | Administering the object |
| Open a file, write one, query a database | The data right |
| Attach to a project | Either, plus administering the project |
| Detach from a project | Administering the project alone |
| See the object in the catalogue, read its permissions | Either |
| Read an extract's frozen file list | The data right on the extract or its study |

Reading is recorded, not only writing. Listing a bucket's contents, opening
an extract's frozen file list and querying a study each write an audit line
with what was asked and how much came back - never the names themselves, so
the journal does not become the leak. Without them the platform could say no
file had been downloaded and nothing at all about who had looked, which is
the first question a study sponsor asks about health data.

Detaching needs nothing on the object on purpose. The same authorisation as
attaching used to guard it, which manufactured a stuck state: revoke the read
on a dataset and nobody could remove it from the project any more, while the
filler kept mounting it. Removing data from a project discloses nothing.

A connector's password is the one credential that travels with the grant. A
secret belongs to one person, so lending a connector would otherwise hand over
a host, a port and no password - a right exercisable nowhere. The borrower's
own secret of that name is tried first, the owner's second.

### Ontology and extract permissions

The same three roles, on the two objects a dataset produces. An ontology is a
reading of a dataset and an extract a selection over an ontology, so each is
owned by the owner of what it describes and each can be lent without parting
with it:

- `owner`: paternity, delete, project assignment and ACL management
- `reader`: read only - which for an extract is everything there is, since a
  frozen selection cannot be written to

Lending an extract grants nothing on the study it was cut from. That is the
point of the object: the filler fetches its files for a workload with no right
to the bucket, so a partner reads 30 tabulated files without being handed the
4,467 images around them.

Every one of these grants appears in the access matrix under Administration,
with its resource type - `dataset`, `ontology`, `extract`, `datasource`,
`project` - so "who can read what" answers for all of them in one place.

The matrix no longer expands an owning organization into one `owner` row per
member for catalogue objects: it did, and the screen an auditor reads asserted
that each of the eighteen people in Essilor held read and write on eight HDS
buckets. The organization keeps its own row, sourced `owner-organization`, and
the shipped role document carries that row too - administration on the data
columns, not RW. A compliance document that overstates access is worse than
none.

### A workspace carries one person's rights

The datasets inside a workspace are the ones its launcher may read, resolved
at the moment it starts. Nothing recorded who that was, so the only thing
guarding the door was the project's launch role: a colleague could open a
running session and read what had been mounted for somebody else, holding no
right over that data at all. It was the widest way around the model.

The workspace now records its launcher, and entering asks for the same right
that filled it - the project role first, then the person. A global
administrator keeps the access they have everywhere, because somebody has to
be able to look at a session that will not start. A row written before the
field exists, or rebuilt from a cluster listing with no record left, carries
no launcher and falls back to the project role: nobody loses a running session
to a migration.

Repositories stay personal. A repository has one owner and no sharing, which
is the decision rather than an omission - so a repository somebody else
attached is not cloned into your workspace.

What a workspace leaves out is now said. Both filters were a silent
`continue`, so a project attaching three datasets started a session with one
and said nothing; you saw an empty folder with no way to tell whether the data
was missing or you were. The launch returns what it withheld, by name, and the
screen shows it.

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

Eight actions are decided against these roles, and they are the vocabulary the
Enterprise role matrix extends:

| Action | `viewer` | `editor` | `admin` |
|---|---|---|---|
| `project.read` | yes | yes | yes |
| `project.launch` | no | yes | yes |
| `project.build` | no | yes | yes |
| `project.manage_members` | no | no | yes |
| `dataset.attach` | no | yes | yes |
| `ontology.attach` | no | yes | yes |
| `datasource.attach` | no | yes | yes |
| `environment.manage` | no | yes | yes |

The last four answered the same question as `project.launch` until the matrix
needed to tell them apart: attaching an extract and starting a workspace were
one permission, so an installation could not say "this role reads extracts and
writes none of them" without also saying it may not run anything. Community
still applies the same rule to all of them; what changed is that the question
names the resource.

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

## Sessions and tokens

Three numbers, and they answer different questions. `scripts/keycloak/harden-realm.sh`
sets all three, so a realm rebuilt from `bootstrap-realm.sh` does not quietly
return to Keycloak's defaults.

| Setting | Value | What it decides |
|---|---|---|
| `accessTokenLifespan` | 30 min | How long a bearer token stays valid before the interface refreshes it |
| `ssoSessionIdleTimeout` | 4 h | How long a session survives with no request at all |
| `ssoSessionMaxLifespan` | 10 h | The ceiling: a fresh sign-in every day |

The access token was Keycloak's default of five minutes until 2026-09-16, and
that is invisible on a good link: the interface refreshes on the next request.
On a bad one it is not. Every brief interruption fell inside a refresh window,
the refresh failed, and the session ended - a user on a corporate network that
drops now and then reported losing the platform periodically. Thirty minutes
makes an unstable link survivable without changing how long a session lasts.

The interface also stopped treating the two failures as one: a refresh Keycloak
*rejects* ends the session, a refresh that never reached Keycloak is retried
once before anything is concluded. Signing somebody out because their network
blinked is the platform punishing them for their building.

"Nothing happening" means no request, not no human, which is why the idle
timeout is four hours rather than Keycloak's thirty minutes: reading one screen
for half an hour used to end the session and put the next click on a sign-in
page.

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

Enterprise adds two things to the model above: an installation may describe
roles of its own, and a stored matrix decides what every role may do.

The matrix existed as a document for months while nothing read it back —
Community answered every question from its own rule and the Enterprise hook was
never supplied, so the screen described a platform rather than governing one.
That is the arrangement ADR-034 forbids, and what follows is what actually
decides today.

### The document

One row per role, one column per part of the platform:

| Column | What it governs |
|---|---|
| `project` | reading the project, and managing its membership |
| `dataset` | attaching and detaching datasets |
| `ontology` | attaching, detaching and scanning ontologies |
| `datasource` | attaching and detaching datasources |
| `environment` | managing environments |
| `workload` | launching workspaces, jobs, apps; running builds |
| `governance` | the platform administration screens |

Each cell holds one of `-`, `R`, `RW`, `Admin`, `R attaché`, `RW attaché`. The
`attaché` variants mean the same access restricted to resources attached to the
project; the restriction is a property of which resources are visible, not of
what may be done to them, so it is enforced where attachment lives.

Actions map to columns explicitly, in `rbacActionColumns`, rather than being
derived from an action's name. An earlier version read the label out of an error
message, so renaming "app restart" to "app rebuild" would have silently
re-classified it.

### Rows the platform ships

Seven rows describe what the platform already does, and they are **locked**: the
API refuses a change to them, and the engine reads the shipped definition rather
than an installation's stored copy. That copy ages — an installation that saved
its matrix before a row changed keeps the older wording, and obeying it withdrew
a right the platform still grants. What a customer wrote stays exactly as
written; the rows describing the platform come from the platform.

The shipped rows reproduce the Community rule exactly. A test walks every
built-in role against every action and compares the two answers, which is what
makes enforcement safe to turn on: an installation that never touched its matrix
sees no verdict move.

### Roles an installation adds

A custom role is held like any other — `data-steward`, not `editor` — and it
**always answers as a built-in**, named in its `basedOn` field.

That base is what every rule written in Go reads, and what Community answers
with where the matrix decides nothing. It is also a ceiling: a role based on
`viewer` may be described as doing anything and, until the matrix is enforcing
the row, it views. A row that declares no base answers as `viewer`, so a
document written before the field existed cannot silently widen anybody.

Combining a direct grant with an organization's compares the two **bases** and
returns the grant as held: the matrix has to see the role that was actually
given, and ranking a custom role as unknown would have quietly capped the person
it was meant to widen.

`GET /api/v1/roles` answers what may be granted here — the three built-ins
always, the installation's own where this edition can enforce them. A role the
platform would not honour is left out rather than offered and refused on save.

### Governance is refused, not granted

`governance` is the column an installation would most like to fill in and the
only one the platform cannot honour. A role is held **inside a project**; the
administration screens are not inside any project, so no project role can
honestly grant them — and a matrix that could promote its own editor to platform
administrator would be a matrix anybody reaching that screen could use to take
the installation.

Two honest options remain: decide, or stop offering. It cannot decide, so the
API refuses a custom row carrying governance and the editor does not offer the
column. Who administers the platform stays with the identity provider.

### Regulated datasets

Attaching an HDS dataset to a project was a global administrator's decision and
nobody else's. That is a queue rather than a safeguard, and the safeguard it
stood in for is entitlement — so both halves are now asked directly:

- the dataset is owned by an **organization the caller belongs to**, and
- the caller **administers the project** it is being mounted into.

Either half alone refuses, and a global administrator keeps the right they
always had. A person cannot own regulated data at all, which registration
refuses at creation.

### Where the code is

| Piece | File |
|---|---|
| Built-in roles and their ranking | `backend/internal/domain/access/role.go` |
| Actions and the Community rule | `backend/internal/http/handlers/access.go` |
| The document, its rows and validation | `backend/internal/http/handlers/admin_rbac.go` |
| Custom roles, bases and assignability | `backend/internal/http/handlers/rbac_custom_roles.go` |
| The engine that decides | `NoryxLab-EE/overlay/.../ee_rbac_provider.go` |

## Notes

- CE remains simple by design and avoids role proliferation.
- EE keeps CE compatibility while adding enterprise-grade delegation.
- Backend authorization must stay the source of truth; the interface only
  reflects capabilities — every screen that hides a control asks the backend
  first rather than re-implementing the rule, because a second implementation
  eventually disagrees with the first and the user is the one who finds out.
