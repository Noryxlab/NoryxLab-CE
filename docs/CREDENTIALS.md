# Who calls the API, and with what

Three kinds of credential exist, and the difference between them is not their
strength - it is **what they authenticate as**. Choosing the wrong one is how a
production ends up owned by somebody who has left, or how a pipeline holds more
authority than the person who wrote it.

| | Personal API token | Service account token | Component credential |
|---|---|---|---|
| belongs to | a person | an account that is not a person | the platform itself |
| acts as | that person | that account | the platform |
| rights | its owner's, narrowed by scope | the account's, narrowed by scope | global administrator, narrowed by scope |
| appears in the audit as | the person's name | the account's name | the component's name |
| created by | its owner, from their account screen | an administrator | an administrator |
| survives its author leaving | no | **yes** | yes |
| can own a project, a dataset | n/a - its owner does | **yes** | no |

## Choosing

**If the task fits inside one person's rights, use a personal token.** A
notebook, a report, a script that reads the projects you are a member of. It is
the weakest of the three by construction, which is the reason to prefer it.

**If the task is a production that must outlive the people who set it up, use a
service account.** A nightly publication, an agent, an application that belongs
to a customer rather than to whoever launched it first. This is the answer to
the question the platform could not answer before: *an app belongs to FOR and
goes into production, the person who launched it leaves - who at FOR answers
for it?*

**Component credentials are not for customers.** They are how the platform's
own parts authenticate: the backup runner, the validator, the restore
rehearsal. They act as the platform, which is why they are administered and why
nothing else should use one.

## Using one

All three are presented the same way:

```sh
curl -H "Authorization: Bearer noryx_<id>_<secret>" \
  https://<your-installation>/api/v1/projects
```

The secret is shown once, at creation, and stored only as a hash - a copy of
the database is not a copy of your credentials. Losing it means replacing it,
which is one click: **Renouveler** issues a replacement with the same scope and
expiry and revokes the original, in that order and server-side, so a failure
never leaves two live credentials for one thing.

## Scopes

A scope can only ever refuse. It narrows a credential below what its holder
could otherwise do, and never widens it.

| Scope | What it permits |
|---|---|
| `read` | every read the holder could make, and no write |
| `datasets` | writes under `/api/v1/datasets` |
| `workspaces` | writes under `/api/v1/workspaces` |
| `jobs` | writes under `/api/v1/jobs`, `/builds`, `/cronjobs` |
| `operate` | the operational admin endpoints: backups, restore, health, validation |
| `invoke` | deployed applications and dashboards, **and nothing else** - not even reading a project list |
| `full` | everything the holder can do |

Reads are permitted by every scope except `invoke`, which is deliberately the
exception: an invoke token is handed to another system, which has no business
listing projects.

`full` narrows nothing. On a service account or a component credential, it is
the most powerful credential the platform can issue.

## Service accounts

A service account is an account in the same directory as a person, marked with
a realm role, and it differs from a person in three ways that matter:

- **It cannot sign in.** No password, no reset, no browser. An account that can
  sign in interactively is a shared password with extra steps.
- **Somebody answers for it.** The responsible person is recorded on the
  account and required at creation. A non-human principal that can own
  regulated data and answers to nobody is the most convenient way to hold an
  HDS dataset nobody is accountable for.
- **It is marked everywhere its name appears.** The audit treats it like any
  other actor, which is honest; a screen that let a reader mistake it for a
  colleague would not be.

Otherwise it is an ordinary account: it belongs to an organization, joins
teams, holds project roles, and owns projects, datasets and ontologies.

Disabling one revokes its credentials first and then the account - an account
disabled while its tokens still work is not disabled - and disables rather than
deletes, because removing the actor breaks the audit trail the moment somebody
asks what it did. What it owned stays owned by it until somebody transfers it,
deliberately.

### The rule that bounds all of this

**Nobody creates a principal more powerful than themselves**, and the check
belongs at creation *and* at every later widening - the cheap way around it is
to create a modest account and then grow it.

Today the platform satisfies this by construction rather than by checking:
every place that creates a principal or hands out a role already demands the
top authority over the thing handed out. It holds because there is no delegated
administration yet. The day an organization administrator can equip their own
organization, this rule needs an explicit check, and
`no_escalation_test.go` fails to say so.

## What is recorded

Every credential records when it was created, when it was last used, when it
expires and when it was revoked. "Never used" is a state of its own on the
screens, because it is the one that says a credential was created and
forgotten - seven such credentials were found on one installation in September
2026, one per redeploy, none revoked, none used for eleven days.

See also: [RBAC_MODEL.md](RBAC_MODEL.md) for what a role permits,
[ORGANIZATIONS.md](ORGANIZATIONS.md) for how organizations and teams are
administered, and ADR-039 in the NoryxProject repository for why a service
account is a principal rather than a credential.
