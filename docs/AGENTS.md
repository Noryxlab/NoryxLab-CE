# Agents

An agent is a standing instruction a user has left with the platform, written
in their own words, and the record of what came of it.

```
Tu surveilles mes workspaces. Quand l'un ne démarre pas,
tu me dis lequel et pourquoi, en une phrase.
```

That sentence is the whole configuration. There is no condition to compose, no
threshold to set and no expression language, and this is deliberate: the people
who know what is worth watching are not the people who enjoy writing rules, and
every monitoring product that asked them to has ended up unused.

Two consequences shape everything below.

**The mission stays free text.** Nothing parses it. The moment a field
constrains what may be said, the writer starts writing for the field.

**What an agent may *do* is never in the mission.** Text cannot be a
permission, because the person writing it is also the person the permission
protects. Actions are a separate, closed list.

## What an agent carries

| Field | Meaning |
|---|---|
| `mission` | What the user wrote. Reaches the model as instructions, never interpreted. Up to 4000 characters. |
| `schedule` | `manual`, `hourly` or `daily`. Three, in the vocabulary of describing a colleague's hours rather than a cron expression. |
| `actions` | From the closed list below. Empty — the default — means it only looks and reports. |
| `projectId` | **Required.** The project the agent works in. See [Scope](#scope). |
| `teamId`, `role` | The group it works in and what it is for there. See [Teams](#teams). |
| `enabled` | A paused agent is never due. |
| `lastQuiet` | Whether the last run found nothing. An agent that is working and has nothing to say looks exactly like a broken one unless the difference is stored. |

### The closed action list

`restart_app` is the one thing an agent may change.

It was chosen because it is reversible and already routine: an application that
has fallen over is restarted, which is what a person would do, and the worst
case of doing it wrongly is an application that restarts when it did not need
to. Anything that destroys state, spends money, or reaches outside the platform
is not on this list and does not get there by being asked for nicely in a
mission.

The list is filtered on the way in **and on the way out of the store**. A row
edited by hand, or written by a version that knew an action this one has
withdrawn, grants nothing.

## Scope

An agent works **in a project**, and this is not a label. The project is the
centre of Noryx: a workspace runs in one, an application is deployed in one,
data is attached to one. An agent is a workload like those, so it belongs to
one too — and every rule that follows becomes a property of the project rather
than a second permission system built for agents alone: what it may see, what
it may act on, what its runs cost, and which data it will read.

The project travels in the credential signed for each run, not as a tool
argument. That distinction is the whole point. The tools filtered on a
`projectId` the model supplied, which meant an agent placed in a project saw
everything its owner saw and could widen its own reach by leaving the argument
out. A scope the model chooses is not a scope.

Two checks apply to anything an agent names, and both are needed:

- **membership** answers "may this person touch it at all"
- **scope** answers "is this what the agent was placed to work on"

Without the second, an agent could name any identifier its owner happens to
reach, and the project would be decoration again.

A person who wants something across all their projects has the platform
assistant, which is that surface and is deliberately unscoped — its credential
carries no project. The two no longer overlap:

| Surface | Belongs to | Reach |
|---|---|---|
| Platform assistant | A person | Everything they can see |
| Agent | A project | That project |

## How a run works

The scheduler wakes every five minutes and runs whatever is due. It starts with
the routes rather than with a page: an agent keeps its hours whether or not
somebody has the screen open, and a worker that only ran while somebody watched
would make "toutes les heures" a promise the platform does not keep.

One run is a conversation the platform has with itself on the owner's behalf,
bounded at **six rounds** — a loop that cannot end is a loop that spends
somebody's GPU all night.

Three things make it different from the assistant's chat loop:

**Nobody is watching**, so the agent cannot ask a question. A run either
produces a report or fails, and "I need more information" is a failed run
rather than a turn. The instructions say so.

**Nothing it does may exceed its owner.** Its tools run through the same
endpoint the assistant uses, under a credential the backend signs for the
owner and that lives two minutes. An agent watching a project its owner has
left stops seeing it the same day.

**An action is not a sentence.** The model can only change something by calling
an action tool the agent was granted; the platform re-checks the grant when the
call arrives; and what actually happened is recorded as a list. The interface
shows that list rather than the prose — the report is text from a model and can
say it restarted something it never touched.

### What a run produces

A report, the list of actions actually taken, and a timestamp. A run that found
nothing worth reporting is marked quiet and kept: a row of quiet hours is how a
reader knows the agent was there, and hiding them would give an empty page for
an agent working perfectly.

## Tools

An agent reaches the platform through a closed set, all scoped to its owner:

| Tool | Answers |
|---|---|
| `get_current_context` | Who this is, which organisations and projects |
| `list_projects` | The owner's projects |
| `list_workspaces` | Their workspaces and state |
| `diagnose_workspace` | Why one will not start |
| `list_apps` | Their applications and state |
| `restart_app` | The one action, refused unless the run's credential carries the grant |

The grant is checked **at the endpoint**, not in the component that asked. A
component deciding whether it is allowed to do a thing is not a permission, and
the model that asked for it is the least qualified party in the exchange.

## Talking to an agent

`POST /api/v1/agents/{id}/ask` is the same agent with a person in the loop. Its
mission is still the instructions, its grants are still its tools, and its
recent journal is the context — so "pourquoi tu n'as rien dit hier ?" is
answered from what it actually saw rather than from a fresh look around. A
quiet hour is shown to it as a quiet hour, because "I looked and found nothing"
is a different answer from "I was switched off".

Two rules make it safe.

**A conversation never widens what an agent may do.** The credential signed for
the exchange carries the agent's own actions and no others, and the tool
endpoint re-checks them exactly as it does for a scheduled run. Asking nicely
is not a permission, for the same reason a mission is not one.

**The exchange is kept in the agent's own journal**, not in a chat history
beside it. An agent's journal is the record of what it did and why; a
conversation held somewhere else would be a second history, and the first
question asked about an action is what prompted it. A failed exchange is kept
too — an agent that looks silent for an hour is easier to understand when the
question that failed is there.

An exchange is bounded at eight rounds rather than the six a scheduled run
gets: somebody is waiting and will ask again if the answer is thin, and a
conversation that stops mid-thought is worse than one that took a moment.

## Teams

Several agents on one subject need what any group of colleagues needs: a
purpose they share, a name for what each one is for, and somebody who can ask
the others to do something. The last part is the difficulty, and it is not
solved by letting them talk.

Most platforms deploying agents stop at the conversation: two agents exchange
messages and neither can establish what the other is allowed to do. The message
arrives and the receiving side either does as it is told — the sender having
borrowed authority nobody granted it — or refuses on a rule nobody wrote down.
A protocol moves that problem rather than answering it.

### Roles are a ceiling

| Role | May |
|---|---|
| `observer` | Look and report. No action, whatever was requested. The default, because an agent nobody thought about should be the harmless kind. |
| `operator` | Take the actions it was granted, on its own schedule. |
| `lead` | What an operator may, and ask other members of its team to act. |

The ceiling is applied **where actions are stored**, not argued about when they
are used. An observer with a granted action is a record that will eventually be
read by something that forgets to check the role, so the grant does not survive
being written down.

Agents stored before teams existed carry no role. Rather than migrate them, the
role is derived the way creation derives it — holding an action means being an
operator — which makes such an agent a member and never a lead, because nobody
promoted it.

### Mandates: the organisation, written in advance

A lead does not decide during a run that it needs somebody's help. A person
writes the organisation down beforehand:

```
Charlotte peut demander à Camille de relancer une application arrêtée.
```

The alternative — letting a lead decide mid-run — is more capable and
unauditable in the way that matters: you would have to read run logs to learn
what the organisation permits, and the answer would be different tomorrow.
Written in advance, this screen *is* the answer to "who may do what".

**A mandate authorises; it never widens.** Both sides must still hold the
action, so writing one cannot grant an agent something its owner never gave it.
Without that ordering the organisation becomes a second permission system
competing with the first, and the two disagree the first time somebody edits
one of them.

A delegation is refused for one of these reasons, checked in this order:

1. the asking agent is not a lead
2. the two do not answer to the same person
3. they are not on the same team
4. the platform does not implement that action
5. the member is switched off
6. the lead was not granted that action
7. the member was not granted that action
8. nobody wrote down that this lead may ask this member for that

The written mandate is checked **last** on purpose. Everything above it
describes something writing a mandate would not fix; reaching that line means
the organisation is the only thing missing, so the message sends somebody to
write exactly the line that will work. Writing a line that could never succeed
is refused at the moment of writing, so the document and the platform cannot
disagree.

Disbanding a team releases its members rather than deleting them. Destroying
standing work because somebody tidied up a grouping is not a tidy-up.

## Where each rule lives

| Concern | Edition | Why |
|---|---|---|
| Agent, run, team, role, mandate, delegation rule | Community (`internal/domain/agent`) | The model is the product's vocabulary and belongs where it can be read. |
| Scheduler, runner, tool endpoint, signed credentials | Enterprise | Community must not carry a runtime for a capability its binary does not serve. |
| API and screens | Community UI, Enterprise routes | `agentsAvailable` answers false where the runtime is absent, and the interface hides the section rather than showing a fault for something never installed. |

## What agents deliberately cannot do yet

Documented because a gap nobody wrote down is a gap somebody will assume is
closed.

**An agent cannot read data.** None of its tools reaches a dataset, a
datasource or a file. It can say a workspace is down; it cannot say a dataset
grew, a column is empty, or summarise a table.

That second gap had a prerequisite, and it has landed. The model gateway
enforces its perimeter policy per gateway project, and every agent on an
installation reaches it with one key — so regulated work used to be
indistinguishable from any other. A run now carries two headers:

| Header | Names |
|---|---|
| `X-Llmaas-Subject` | The workload: `chat`, `developer`, or `agent:<id>` |
| `X-Llmaas-On-Behalf-Of` | The Noryx project the run belongs to |

The gateway resolves the named project's policy and **narrows** the key's to
it. The declaration can only ever restrict: a caller naming a project is
volunteering a constraint, never claiming one, and a named policy that
permitted more changes nothing. Otherwise a header would be a privilege, and
anybody holding any key could send anything anywhere by naming the right
project. A project the operator has not described narrows nothing, so
declaring projects does not refuse every caller the day it starts.

What remains before agents may read data is therefore the reading itself:
tools that list what is attached, describe a schema, and sample a table. An
agent working on a project the operator has marked as staying inside the
perimeter is already pinned to an on-premise tier or refused, and that refusal
is visible in the gateway console.
