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

An agent is what a person sees. What actually executes is a **workflow**: one
reasoning step for the agent above, or several in a written order with a person
among them. The two are the same object seen from two sides (ADR-046), and
nobody has to learn the second to use the first.

If you are here for one thing in particular: what an agent may change is [the
closed action list](#the-closed-action-list), and the grant that deserves a
careful read before you give it is [`call_api`](#what-call_api-opens).

## Jobs or agents

Answer this before anything else on this page.

Noryx already automates: a **job** runs on a schedule, does what its code says,
and produces an output. It is deterministic, repeatable, and auditable by
reading the code. Most of what people first describe as "something for an
agent" is a job.

An agent is not a better job. It does a different thing:

**A job automates a procedure you can write down. An agent handles what you
cannot write down.**

Three things a job structurally cannot do:

**Read the unstructured.** A job parses a CSV. It does not read a free-text
field, a report, or an error nobody has seen before. That is where the variety
lives, and the variety is why the rule could not be written.

**Decide that something is not worth reporting.** A job emits everything, or
everything past a threshold. An agent can answer "nothing worth your
attention" — which is the commonest answer and a different one from silence.
The platform stores that difference: a quiet run is kept and marked, so a row
of quiet hours is how a reader knows the agent was there.

**Compose steps nobody planned.** A job runs a fixed sequence. An agent looks,
then decides what to look at next, up to a bounded number of rounds.

### If you can specify it, use a job

An agent in a job's place costs more, fails more often, and audits worse. That
is the opposite of what the market says about agents, and it is what a risk
function needs to hear first — the credibility of everything else on this page
depends on saying it.

### The question that produces answers

"What would you automate with an agent?" produces nothing. People hear
*automation*, think *job*, and they are right to.

The question that produces answers is: **what do you read, regularly, in order
to decide something?**

That is the shape of the thing. Training output somebody scans to see whether a
run converged; batch logs somebody opens to find why three volumes out of two
hundred failed; a weekly validation report somebody compares against last
week's. Read-in-order-to-decide.

### The filter that stops the bad ideas

Before either: **will you be able to check the answer?**

If nobody can verify the work, it is not a job and not an agent — it is a risk.
An agent whose output nobody can check is worse than no agent, because it
produces confidence rather than information.

| | Use |
|---|---|
| You can write the rule | A job |
| The input varies; somebody reads and judges | An agent |
| Nobody could check the answer | Neither, not yet |

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

Two entries. A name that is not in `internal/domain/agent` does nothing, and
adding one is a reviewed change to that file rather than a sentence in a
mission.

| Action | What it grants |
|---|---|
| `restart_app` | Restart an application that has stopped. |
| `call_api` | Use the platform's own API, as the owner. Read [What `call_api` opens](#what-call_api-opens) before granting it. |

`restart_app` was the first and for a long time the only one, chosen because it
is reversible and already routine: an application that has fallen over is
restarted, which is what a person would do, and the worst case of doing it
wrongly is an application that restarts when it did not need to.

`call_api` is a different kind of grant, and the difference is the point. The
verbs an agent needs — launch a job, write an object into a dataset, declare an
extract, stop an application — already exist as endpoints, each with its access
check written once and its audit entry. Describing them a second time as agent
tools would be a second API kept in step by hand, so the grant is the API
itself (ADR-046). It is also, by construction, much wider than `restart_app`,
which is why it has its own section below.

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
| Agent | A project | That project, **through its reading tools** |

The second row carries a qualifier that must not be read past. The scope is
enforced by the reading tools, each of which calls `withinScope` before
answering. **`call_api` does not go through them**, and therefore is not scoped
to the project — see the next section but one. Until that gap is closed, a
project bounds what an agent *sees*; it does not bound what an agent *does*.

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

An agent reaches the platform through a closed set, all carried out under the
owner's identity:

| Tool | Answers | Grant |
|---|---|---|
| `get_current_context` | Who this is, which organisations and projects | — |
| `list_projects` | The owner's projects | — |
| `list_workspaces` | Their workspaces and state | — |
| `diagnose_workspace` | Why one will not start | — |
| `list_apps` | Their applications and state | — |
| `read_audit` | Who did what, when and from where — platform administrators only | — |
| `list_api_operations` | What the platform's API declares, filtered by a word | — |
| `describe_api_operation` | One operation in full: parameters, body, answer | — |
| `restart_app` | Restart a stopped application | `restart_app` |
| `call_api` | Call one API operation as the owner | `call_api` |

The two description tools read the same OpenAPI document a person reads at
`/swagger`. What an agent is told the platform can do is therefore what the
platform documents it can do — one source, rather than a catalogue of tools
that drifts from the API it describes. Both only read, and need no grant: an
agent that cannot look an operation up is an agent that invents a path, and
inventing identifiers is the behaviour every description on this list exists to
prevent.

The grant is checked **at the endpoint**, not in the component that asked. A
component deciding whether it is allowed to do a thing is not a permission, and
the model that asked for it is the least qualified party in the exchange.

## What `call_api` opens

Granting `call_api` is not granting a bigger tool. It is handing an agent the
console's own API under its owner's name, and the honest description of its
reach is this: **an agent granted `call_api` can do what its owner can do.**

How it works, because the mechanism is what bounds it:

- The assistant names a method and a path. It never reaches the network itself.
- The platform dispatches that request **in-process, through its own router**,
  with the owner's identity placed in the request context — a value nothing
  arriving over the network can set.
- The handler that answers is therefore the handler a browser would reach, with
  its own access check, its own validation and its own audit entry.
- The mutation is recorded against the owner, from the address `agent`, with
  the method and path in the entry.

Four bounds hold today:

| Bound | What it stops |
|---|---|
| `/api/v1/` only | Nothing outside the API: no `/swagger`, no static files |
| `/api/v1/assistant/*` refused | A tool that could call the tool endpoint, which is a loop holding a credential |
| 64 KiB request, 128 KiB answer, 30 s | An agent uploading a dataset through a tool loop, or reading 40,000 objects into its context |
| The handler's own check | Everything a person could not do either |

And three limits that must be read before granting it:

**It is not scoped to the agent's project.** The reading tools apply
`withinScope`; this path does not. An agent placed in project A, whose owner is
also a member of project B, reaches project B. The project still decides what
the agent *sees* through its other tools, and still carries the model
perimeter — but it does not bound `call_api`. Per-path scoping is the first
guard rail on the list, and it is not built yet.

**It reads data.** `GET /api/v1/datasets/{id}/objects` lists a dataset's files
and `GET /api/v1/datasets/{id}/objects/{path}` returns one. On an installation
holding regulated data, that is the sentence to read twice. The dataset's own
access check applies — the agent reads what its owner may read, and nothing
more — but "an agent cannot reach a dataset" stopped being true the day this
grant shipped.

**An agent owned by the bootstrap administrator is a platform administrator.**
The identity synthesised for the call carries the owner's name and no directory
roles, so an ordinary owner's agent is refused on the administration routes.
But `NORYX_BOOTSTRAP_ADMIN_USER` is matched **by username**, so if that account
owns the agent, every global-administrator check answers true — the audit
trail, every organisation, every dataset on the installation. This is
consistent: the agent acts as its owner, and its owner is an administrator.
It is also not what anybody guesses from "it may use the API as you", which is
why it is written here.

The practical rule follows from all three: **grant `call_api` to an agent
owned by an account that holds only what the agent needs.** A service account
scoped to one project is the right owner for an agent that acts; a platform
administrator is the wrong one.

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

## Workflows

An agent is what a person sees. A **workflow** is what executes (ADR-046).

The two are not competing abstractions. An agent as described above — one
instruction, one schedule — *is* a workflow of a single reasoning step, and
nothing anybody has recruited changes shape. What a workflow adds is a
sequence: several steps in a written order, where one of them may be a person.

```
Veille puis analyse            toutes les heures
  1  agent       Cherche ce qui est nouveau sur le sujet.
  2  approval    Relecture — attend Camille
  3  agent       Analyse ce qui a été trouvé.
```

A sequence, deliberately, and not a graph. It is the pattern the reference
recommends where control matters — *"regulatory compliance, financial
transactions"* — and it is also the only shape a person writes correctly
without learning a tool. There is no branch, no condition and no expression
language, for the same reason a mission has none.

### What a step is

| Kind | Does | Carries |
|---|---|---|
| `agent` | One reasoning step, run exactly like a single agent: instructions, tools, bounded rounds | `instruction`, `actions` |
| `approval` | Waits, durably, for one named person to let it through or stop it | `approverUserId` |

Up to **twelve** steps. A workflow longer than that is several workflows, and
saying so at creation is kinder than discovering it at three in the morning.

An agent step is granted actions from the same closed list, checked at
creation with the same gate an agent passes: **a step cannot be granted what
its owner could not grant an agent.** An approval step names a person who must
be a member of the project — otherwise the run parks on somebody who will
never see it.

### What one step hands to the next

The previous step's report, appended to the next step's instruction under a
visible heading:

```
Analyse ce qui a été trouvé.

--- What the previous step produced ---
Trois publications nouvelles depuis hier, dont deux sur …
```

That is the whole hand-off. The steps do not talk to each other, do not know
each other exists, and share nothing but the run. Free conversation between
agents is excluded on purpose: on regulated data, emergent behaviour is a
defect rather than a feature.

### Runs are durable

A run is written to the database **after every step**, which is what makes a
backend restart cost nothing: the row is the checkpoint and the process that
died is not missed. On the next sweep the scheduler picks up unfinished runs
and walks each from its first unfinished step.

| Status | Means |
|---|---|
| `pending` | Written, not started |
| `running` | A step is executing |
| `waiting_approval` | Parked on a person. Nothing is held open; it can wait a week |
| `succeeded` `failed` `cancelled` | Terminal |

A failed step is retried, up to **three attempts**, on the following sweep
rather than in a loop — which gives an assistant that was briefly unreachable
five minutes to come back before the second attempt is spent. Each attempt
carries a distinct idempotency key (`workflow-<run>-step-<n>-attempt-<k>`), so
a retry is never mistaken for the same call arriving twice.

Each step records what it produced, how many attempts it took, and **the list
of actions the platform saw** — never what the report claims. The run timeline
shows that list, for the same reason the agent journal does.

### Approving

An approval step shows, in the run's own timeline, what the step before it
produced, with the two buttons beside it — because the decision is taken by
reading that, not in an inbox somewhere else. The platform refuses a decision
from anybody but the named approver, and refuses one on a run that is not
waiting.

Approving walks the run onward immediately rather than waiting for the next
sweep: somebody is looking.

### What workflows deliberately do not have

- **No generic nodes.** No HTTP node, no script step, no JSON node. A step is
  a reasoning step or a person. Something the platform cannot do is a missing
  verb, to be added with its policy, not a hole to route around.
- **No canvas, yet.** The interface draws the sequence — trigger, steps,
  report — from the definition and the latest run. Composing *by* drawing is
  the fourth phase of ADR-046, and it is worth exactly what it depicts.
- **No triggers beyond the three schedules.** `manual`, `hourly`, `daily`.
  Saying when in words, and starting from a platform event, are phase 2.
- **Mandates are not yet edges.** Teams and mandates below are written and
  displayed; the runner does not read them.

## Operating them

Everything here is Enterprise, and all of it is in the backend — no external
workflow engine, no second component to operate on an air-gapped site.

| | Behaviour |
|---|---|
| Sweep | Every **5 minutes**, for agents and workflows alike. Resumes unfinished runs first, then starts what is due |
| Concurrency | **One run advances at a time** across the installation, under a process-wide lock. Scheduled work never queues the people who are present |
| Bound per advance | **10 minutes**. A run that exceeds it is parked by its own failure and retried, not held |
| Rounds | 6 per scheduled agent run, 8 for an exchange with a person |
| Tables | `agents`, `agent_runs`, `agent_teams`, `agent_mandates`, `workflows`, `workflow_runs` |
| Credential | Signed per run, lives **2 minutes**, carries the owner, the project and the granted actions |

What the log says, and what to do about it:

| Line | Means |
|---|---|
| `workflow scheduler started (every 5m0s)` | The runtime is up. Absent at boot means `workflowStore` is nil — the migration did not run |
| `workflow <name> (<id>) is attached to no project and was not run` | A workflow with no project cannot be governed from any screen, so it does not get to act. Attach it or delete it |
| `workflow run <id> belongs to no workflow and was not resumed` | The definition was deleted under a live run. Said rather than cleaned up: the row is what makes it visible |
| `workflow <name> (<id>) could not start: …` | The assistant or the gateway refused. Check `noryx-assistant` first, then the model gateway |

A run stuck in `running` for longer than the ten-minute bound is a process that
died between two writes: the next sweep picks it up from its last written step.
A run in `waiting_approval` is not stuck — it is waiting for the person named
on the step, and the decision inbox on the agents page lists every one of them.

Three things worth watching on an installation that uses them: the **number of
agents that may act** versus the number that exist (the administration page
separates them), the **runs waiting on a person** (a decision nobody takes is a
workflow nobody finishes), and the **model spend per agent**, which the gateway
attributes from the `X-Llmaas-Subject` header each run carries.

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

## Answering for them all at once

An installation running agents gets asked what they may do — by its own risk,
legal and quality functions, and the person asked is rarely the person who
built them. Answering meant opening every agent and every team one by one,
which nobody does.

The administration section answers it on one page, under **Agents**
(`/admin/agents`, Enterprise), and exports the same as CSV, because the answer
usually has to leave the platform to reach a committee.

It answers three questions:

**Who they are, and what each may do.** Every agent, its project, its team, its
role, whether it may act or only look, its rhythm, and when it last ran. The
summary counts how many **may act** separately from how many exist — that is
never the same number, and it is the one a risk function asks for first.

**Who may ask what of whom.** The written organisation, rendered as sentences:
*Ariane may ask Atlas to restart a stopped application*, with who allowed it
and when. A line reads without knowing the word delegation, and a line that is
not there is a thing that cannot happen — see [Mandates](#mandates-the-organisation-written-in-advance).

**What they actually did.** Runs and actions over a window.

The activity figures come from the **list of actions the platform recorded**,
never from the reports. A report is text a model produced and can claim an
action that never happened; the list is written by the platform when it
performs one. This is the same rule the journal follows, and it is the reason
the page can be put in front of an auditor.

## Where each rule lives

| Concern | Edition | Why |
|---|---|---|
| Agent, run, team, role, mandate, delegation rule | Community (`internal/domain/agent`) | The model is the product's vocabulary and belongs where it can be read. |
| Workflow, step, run, status, retry and approval rules | Community (`internal/domain/workflow`) | Same reason. The rules are readable, and testable, without a runtime. |
| The in-process API surface — index, description, dispatch | Community (`internal/http/handlers/api_surface.go`) | It is the platform calling itself through its own router; nothing about it is Enterprise. |
| Scheduler, runner, tool endpoint, signed credentials | Enterprise | Community must not carry a runtime for a capability its binary does not serve. |
| API and screens | Community UI, Enterprise routes | `agentsAvailable` answers false where the runtime is absent, and the interface hides the section rather than showing a fault for something never installed. |

## What agents cannot do yet

Documented because a gap nobody wrote down is a gap somebody will assume is
closed.

> **This section said, until October 2026, that an agent could not read data.**
> That stopped being true when `call_api` shipped: with the grant, an agent
> reaches `GET /api/v1/datasets/{id}/objects` and the object behind it, under
> its owner's access. The old sentence is kept here, struck, rather than
> quietly deleted — a security property somebody may have relied on deserves to
> be seen changing. Read [What `call_api` opens](#what-call_api-opens).

Five gaps remain, in the order they matter:

**`call_api` is not scoped to the agent's project.** The reading tools are;
this path is not. An agent reaches every project its owner reaches. Until that
is closed, the project is not a containment boundary for an acting agent — only
for a looking one.

**There is no per-path guard rail.** The grant is all of the API or none of
it. A policy saying "this agent may launch jobs and write to this dataset, and
nothing else" cannot be expressed. Both this and the previous line are phase 1's
deliberate debt (ADR-046): get the surface right, then narrow it.

**There is no verb for reaching outside.** Nothing fetches a page, calls a
connector or reads a feed. `fetch_external` is designed as a verb gated per
project — exactly as the model gateway gates leaving the perimeter — and it is
not built. An air-gapped installation should have no such verb at all, and
today none does.

**Mandates are declared, not executed.** Teams, roles and the written
organisation are stored, checked at writing time and displayed on the
governance page. No scheduler or runner reads them. The organisation is
written; it does not run.

**Triggers are three schedules.** `manual`, `hourly`, `daily`. There is no
"every Monday at eight", no "when a file lands in SELENA", and no way to say
either in the words a person would use.

### The perimeter, which is not a gap

One thing that *was* a prerequisite has landed, and it matters more now that an
agent can reach data than it did when it could not.

The model gateway enforces its perimeter policy per gateway project, and every
agent on an installation reaches it with one key — so regulated work used to be
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

An agent working on a project the operator has marked as staying inside the
perimeter is therefore pinned to an on-premise tier or refused, and that
refusal is visible in the gateway console.

**This bounds the model, not the data.** The header says which project a run
belongs to, so the gateway can refuse to send that project's prompts to a
provider outside the perimeter. It says nothing about what the agent reads
through `call_api`, and nothing stops a mission from putting a file's contents
into the next prompt. Where the gateway itself sits outside the installation —
as it does for the EMSE deployment, which reaches `www.noryxlab.ai` — the rule
to design against is blunt: **a workload that reads regulated data must not be
the workload that writes the prompt.** Have a job read, aggregate, and emit
counts; have the agent read the counts. The platform does not enforce that
separation today, and the person writing the mission is the one who holds it.
