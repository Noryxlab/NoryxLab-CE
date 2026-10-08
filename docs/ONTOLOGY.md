# Ontology

> Looking for how to use the screens? [USING_ONTOLOGIES.md](USING_ONTOLOGIES.md)
> is the guide for people opening the catalogue. This page is the design
> reasoning and the storage model.


**Where this is today, in one line:** it reads a dataset's object *paths* -
never their content - and infers a first draft of the objects in it: a study,
its pseudonymised subjects, their visits, modalities, formats and volumes.

The name stays. "Ontology" is the category the product is aiming at and the
word a buyer recognises, and renaming the feature to match what it does today
would trade the ambition for accuracy about a version that will not last.

What had to go were the specific promises around it. The empty screen said the
ontology was there "to query the platform in natural language"; the query
filters a manifest by subject, visit and modality, so the screen now says
filter and the field offers `ANTERION` rather than "how many orders were
placed last month". A word that names a direction is fair; a sentence that
describes a capability nobody built is not.

**What makes the word true**, and it is open: declared objects with properties
and links, bound to data, with this scan as their first step rather than their
competitor - the inference proposes Study, Subject, Visit and Modality, and a
person confirms, renames and links them.

The test of whether it is worth building was that it had to turn a selection
into a mount, and that part now exists. Three properties hold it up.

**Freshness.** An ontology is a photograph and the screen presented it as a
fact: the June scan of PREMYOM1000 read "18,738 objects, 20 subjects" in the
same typeface as September's 24,179 and 31, with nothing in between to say the
study had recruited eleven subjects. Opening an ontology now counts what the
source holds today and states the difference in objects. The count is a
listing: the platform reads how many objects are there and writes nothing back.

**Coverage.** "31 subjects" averaged together the subjects who carry a corneal
wavefront and those who do not. Coverage names them - how many subjects hold
each modality, sparsest first, and the identifiers of those who do not - so an
extract's owner learns what it excludes before publishing an n rather than
after.

**What mounts, and what does not.** A dataset is the bytes: a bucket, a
prefix, a credential, mounted raw and read-only. An ontology is the *reading*
of a dataset - what the paths mean and what is inside - and **does not mount**:
it describes a source, it does not contain one. An extract is a view of an
ontology, and it is the only object a workspace mounts. The whole ontology is
the extract with no filter, which is a button rather than a thing to work out.

**Extracts.** An extract is a named selection, resolved to an explicit list of
files at the moment it is declared and kept that way: an extract that re-ran
its filter would return a different study every month. It duplicates nothing.
The frozen paths point into the dataset where the data already lives, and a
workspace mounts the extract as a tree of symlinks at
`/extracts/<name>/<levels>/<file>` over the read-only dataset mount - beside
`/datasets` and `/repos`, because an extract is a read-only view of a bucket
and belongs beside the bucket. The IDE opens it. The file list travels in the workspace bootstrap
secret, which caps an extract at roughly 700 KB of compressed paths; past that
the bootstrap log says the extract was not mounted rather than building a
partial tree that would silently be a different study.

It was called a cohort until 2026-10-01. The word carried a clinical meaning
into screens where the mechanism is simply a subset of a listing, and the
platform is sold outside medicine - ADR-042 records the rename and what it
cost.

Storing the paths is what made extracts possible - the manifest keeps three
sample paths per modality, which shows a person what the data looks like and
could never define a study. Ontologies scanned before this refuse to produce an
extract and ask for a rescan, rather than returning an empty selection that
looks like a legitimate answer.

**Declaring one.** An extract carries two things. A **selection** - a filter
over the ontology's own vocabulary, modalities, subjects, visits, resolved once
and frozen. And a **layout**: the order of the directory levels. Subject-first
answers "what does this patient have"; modality-first answers "show me every
cornea scan I hold". Two questions, one set of files, and until 2026-10-01 only
the first was expressible because the tree was built in a fixed order.

A layout is a permutation of the three levels and never a subset. Dropping one
would put files from different visits in the same directory, where the ones
sharing a name would overwrite each other - a loss that surfaces as an `n`
nobody can reproduce.

The selection itself is a filter over the ontology's own vocabulary -
modalities, subjects, visits - resolved once and frozen. The modalities are
offered rather than typed: they are named and counted from the coverage above,
exact spelling included, which is the part nobody can guess for a study they
did not file. Selecting none means every modality, and the form says so under
the field.

That sentence used to live in the field's label, as "(empty = all)", and on
2026-10-01 an extract declared as `Selena-extract-modality` came back carrying
all 4,023 objects of its ontology because the field had stayed empty - nothing
on the screen named the modalities it could have asked for. A wrong `n` is
discovered months later, in a paper.

**Rescanning.** Scanning a source already described refreshes that ontology
rather than adding a second one beside it: the manifest and the stored paths
are replaced, and the object keeps its identifier, its name, its owner, its
permissions and the extracts declared over it - whose file lists are frozen, so
a listing that changed underneath does not move their `n`. The reply says which
happened, `created` or `updated`, and so does the message on screen.

Only the same source with the same inference profile, and only among the
ontologies the caller can already see: two profiles reading one bucket are two
descriptions, and refreshing an invisible ontology would be editing somebody
else's object through a scan. ADR-043 records this and what it costs - a
refresh is not versioned, so the answer to a bad scan is another scan.

**Correcting a name.** Both an ontology and an extract can be renamed, by their
owner or a global administrator. An ontology's name is read off the source and
comes out wrong - `SELENA-01` for a study its owner calls SELENA-001 - and an
extract's is typed, which is worse. Before this the only way out was to produce
the object again: for an ontology that left a duplicate behind, and for an
extract it froze a *different* list over a bucket that keeps growing, so a
rename became a different study.

The label moves and nothing else does. The frozen file list, the author and the
dates are what an extract is, so an `n` already published against a name still
describes the same files. The name is a label and not a key: objects, extracts,
permissions and project links all hang off the identifier.

An ontology has no status to display. The field exists and only ever holds
`active`; showing it rendered "Running" on every row, which is a word about
execution applied to an object that does not execute. Freshness and coverage
are what can be true or false about an ontology, and they have their own cards.

**How a source is read, and how to change it.** The platform read every bucket
with one rule compiled in: find a segment that looks like a subject identifier,
take the next two as the visit and the modality. A dataset can now carry its
own rule instead - which path level holds the subject, which the visit, which
the modality, counted from zero. For
`SELENA/SELENA-01-001/20260218/ANTERION/DICOM/f.dcm`: subject 1, visit 2,
modality 3. A level left empty means the source does not carry it, which is a
real answer for a study with one visit per patient.

It is positional and deliberately not a regular expression. A rule somebody can
get subtly wrong, over health data, that files objects under the wrong patient
and is discovered months later in a published figure, is a hazard with a text
field - where three numbers can be read aloud and checked against one path
somebody knows.

**Try it before storing it.** The platform applies a candidate rule to a dozen
real keys and shows what it reads - path, subject, visit, modality, and how
many were recognised. Nothing is stored until you say so, and *forget the rule*
restores the compiled one. The trial runs on real keys and is returned only to
somebody who may already read the bucket; it is never sent to a model, which is
why the assistant is shown path *shapes* instead.

The assistant answers in the same vocabulary - "subject: level 1, visit: level
2" - so applying its proposal is a transcription of three numbers rather than
an interpretation of a paragraph. It proposes and never applies (ADR-040): a
person stores the rule. An installation with no assistant fills the three
fields itself, which is the point rather than a consolation.

Changing a rule changes nothing until a rescan - and a rescan refreshes the
ontology rather than adding one, so correcting a reading leaves one ontology,
read properly. ADR-045 records the shape and why it is positional.

**An ontology records the reading that produced it**, not the name of a
profile that may have changed since. So a rescan can say which of two things
happened: "the reading changed - 4,023 to 3,908 objects, 2 to 31 subjects, the
rule explains the difference, not the data", or "the source moved, with the
reading unchanged". Those are different news and they call for opposite
reactions; a subject count that moves without a cause reads as a broken
platform.

An ontology scanned before 2026-10-01 carries no rule, so the comparison gives
the figures and declines to say the reading changed - claiming it wrongly would
send somebody after a rule nobody edited.

**Ownership and attachment.** An ontology and an extract are owned the way a
dataset is - by a user, a team or an organization - and both can be handed on.
A team is the useful case: it is the unit that works together, so it is the
unit that keeps what the work produced when a member leaves.

Attaching either to a project is a link, so several projects may mount the same
ontology or the same frozen file list, and detaching removes the mount rather
than the object. An extract therefore outlives the project that first needed
it, which is the point: the alternative is a second scan over a bucket that has
grown, and an n that no longer matches the first analysis.

Creating an extract does not ask for a project. The scan form asks for a
dataset; the project's data screen is where attaching happens, beside the
datasets and datasources it already attaches.

What a person may **see** in the extract catalogue follows the ontology, not
the extract's own owner: an extract's name and its n describe the ontology's
content, so one drawn from an ontology the caller cannot read is not listed.
Ownership decides the other half - who may transfer or delete it. Being handed
an extract is not being handed the study it was drawn from.

Throughout, the source bucket is read-only to the platform: it is listed, and
never written, copied or modified.


NoryxLab can generate a first semantic catalog from datasets attached to the active project. The UI entry point belongs to the Data domain, not to the project resource panel, while the stored manifest remains project-scoped for RBAC and dataset attachment checks.

The current dataset inference is intentionally profile-based, not generic. The first supported profile is `noryx-file-path-v1`, built for the Noryx/FOR file layout.

## Scope

The `noryx-file-path-v1` profile scans S3 object metadata only and tries to infer:

- a study, for example `PREMYOM1000`
- pseudonymized subjects/patients, for example `PREMYOM1000-0001`
- visits/dates when present in paths
- modalities, for example `ANTERION`, `IOLMASTER`
- formats, for example `CSV`, `PDF`, `XLSX`
- measurement tables from CSV/TSV filenames, for example `Cornea_Basics`
- object counts and byte sizes

For HDS datasets, the scan does not download object content. It does not parse DICOM tags, CSV rows, PDF content, images, or Excel values. On another dataset layout, this profile can produce incomplete or irrelevant output; adding a new domain requires adding a new explicit inference profile.

Datasource catalogs currently use the `datasource-metadata-v1` profile: connection metadata only, without SQL/NoSQL schema introspection. Real DB schema extraction is a later explicit step.

## UI

The catalog is exposed from `Data > Catalogue sémantique`. Users select a source dataset or datasource accessible from the Data domain, then create a catalog. This keeps the product model Palantir-like at the data layer while preserving project-level access control. Generated catalogs are also project resources: they can be attached to or detached from projects through the project resource panel, with the same operating model as datasets.

## API

```http
GET /api/v1/ontologies
GET /api/v1/projects/{projectID}/ontology
POST /api/v1/projects/{projectID}/ontology/scans
GET /api/v1/projects/{projectID}/ontologies
PUT /api/v1/projects/{projectID}/ontologies/{ontologyID}
DELETE /api/v1/projects/{projectID}/ontologies/{ontologyID}
```

Scan payload:

```json
{
  "sourceType": "dataset",
  "datasetId": "dataset-id"
}
```

Datasource payload:

```json
{
  "sourceType": "datasource",
  "datasourceId": "datasource-id"
}
```


## Output Model

Ontologies are first-class Data objects, like datasets:

- owner user or organization
- `owner`, `writer`, `reader` permissions
- source dataset or datasource
- inference profile
- manifest JSON
- project attachments through `project_ontology_links`

The legacy project-scoped manifest endpoint is kept for UI compatibility and “latest scan” display, but the source of truth for sharing and project attachment is the ontology object.

The stored manifest contains:

- project and dataset identifiers
- inference profile
- study name inferred from subject identifiers
- global summary: subjects, visits, modalities, objects, size, formats, measurement tables
- subjects
- visits per subject
- modalities per visit
- sample object paths per modality

## HDS Safety

The ontology MVP intentionally avoids exposing health metadata extracted from file contents. Pseudonymized IDs, visit dates and object paths are still health-context metadata and must be handled as HDS data in Enterprise Edition.

Future healthcare services may add controlled PHI checks, integrity checks and deeper schema extraction, but they must remain explicit and audited.

## What a dataset says about itself

The scan produces an inventory of paths — subjects, visits, modalities, counts,
volumes — and says so honestly. What it cannot produce is what the dataset is
*about*: its purpose, what it must not be used for, the legal basis it was
collected under, the units a measurement is in, who to ask. None of that is
inferable from any number of bytes, so it is **declared** (ADR-047).

`GET /api/v1/ontologies/{id}/card` and `PUT` the same path.

**It lives on the ontology, not on the dataset.** A dataset is a bucket with
credentials; the ontology is the layer that says what is in it, which is where a
description belongs — and a bucket can carry several studies read several ways,
so a card on the dataset would force one description on all of them.

It was put on the dataset first, from a misreading of ADR-043. That ADR says a
rescan "replaces the picture and **keeps the object** — its identifier, its
name, its owner and the extracts that point at it". The manifest is replaced;
the ontology row is not. A card stored beside the manifest survives a rescan
exactly as the name does.

### It is never returned alone

Trust does not exclude verification. Every declared figure the platform can
measure is compared with the most recent scan, and the result is a line per
field — including the fields nobody declared, because a reader who does not see
a field cannot tell it apart from a field that passed.

| Verdict | Means |
|---|---|
| `agrees` | the measurement matches the declaration |
| `differs` | both are known and they do not match — a stale declaration and a wrong reading rule look identical here, and which it is belongs to a person |
| `undeclared` | nothing was stated, so there is nothing to check |
| `unverified` | stated, but nothing has measured it yet |
| `not_checkable` | nothing could measure this, ever — a purpose, a legal basis |

Three properties make this a control rather than a nag. **A check reports, it
never overwrites**: the declaration stays as written and the verdict sits beside
it, because replacing somebody's word with a measurement destroys the only
evidence that they disagreed. **A check carries its method and the date of the
measurement**, not of the comparison — a fresh comparison against a month-old
scan is a month-old answer. And **`not_checkable` is a verdict**: inventing a
proxy for a purpose would be exactly the kind of claim ADR-034 forbids.

### The pseudonymisation claim

`pseudonymised` reads as **`unverified`** until a structure scan has read the
technical fields. That is the honest answer and the reason the field
exists: the survey of `hds-for` on 2026-10-05 had to report 3 592 DICOM headers
unchecked, and nothing in the platform could record that fact.

`PUT /api/v1/ontologies/{id}/structure-scan` records one audited pass, and
`GET` reads the last one. The platform does not launch it: reading inside files
is a different act over data that may be regulated, so a scanner runs under an
allowlist and reports there — and that endpoint is the line in the audit trail
saying it happened.

What arrives is checked rather than trusted. A field listed as both recorded and
identifying is refused: it would be read to be checked *and* read to be kept,
and the second wins by accident. Values for a field that is identifying anywhere
in the allowlist are refused. And a recorded field with more than 64 distinct
values loses them — past that it is the column with extra steps, not a
distribution.

**Measured on 2026-10-07.** `hds-for` (premyom1000), 3 592 DICOM objects, zero
unreadable: ten identifying fields carry a value, including `PatientName`,
`PatientID` and `PatientBirthDate`. `hds-for-selena`, 2 009 objects, zero
unreadable: the same ten. Neither is pseudonymised at the header level. The zero
is what makes those verdicts mean anything — a scan that skipped files could not
conclude.

### Versions

Each accepted edit increments the card version and records who wrote it. An
extract records the version it was cut against, so a cohort's `n` stays
explainable after a later edit — the same reason an ontology records the reading
rule that produced it (ADR-040).

## One source, one ontology

EMSE carried two `PREMYOM1000` over one bucket for a month — 31 subjects and 32
subjects, and nothing to say which was the live one. Two paths led there, and
both are closed.

**A different inference profile no longer separates two ontologies.** It did, so
a rescan with another profile created a twin. One source, one ontology — and the
difference between two photographs is read in the reading rule each one records
(ADR-040), which is exactly what that rule is for.

**An ontology the caller cannot see is still found.** Refreshing only looks among
visible ontologies, deliberately: refreshing an invisible one would be editing
somebody else's object through a scan. But the scan then created a twin instead,
which is worse — two ontologies describing one bucket with nothing saying which
holds. A scan over a source already described by an invisible ontology is now
refused with `409 source_already_described`, and the message says to ask for
access rather than to scan again.

Extracts are the exception, and always were: several extracts over one ontology
is the point of an extract.

**Enforced in the database too**, by a unique index on `(source_type,
source_id)` — because the scan path is what closed this, and the index is what
keeps it closed when somebody adds a second path later. That is the argument for
a constraint over a check.

It could not ship with the guarantee: a failing migration statement stops the
backend from starting, and both clusters carried a duplicate, so the index would
have taken the platform down rather than protected it. Both were resolved on
2026-10-07 — in each case by deleting the older photograph, after moving a live
project's link to the newer one.

**An installation that still carries a duplicate will refuse to start**, loudly,
and that is the right failure: it names a condition somebody has to decide about
rather than guessing which twin to drop. The repair is the one used here — move
any live link to the ontology you keep, then delete the other with its access
rows, its project links and its object rows.

### Deleting an ontology deletes its object rows

`DeleteOntology` removes the project links, the access rows and the ontology,
and now `ontology_objects` explicitly.

**The explicit delete changes nothing in Postgres**, and the earlier claim here
that it leaked was wrong: `ontology_objects.ontology_id` is declared
`REFERENCES ontologies(id) ON DELETE CASCADE`, and the constraint is live on
EMSE (`confdeltype = 'c'`). The database was already removing them. It is
written out anyway because the statement should say what it means — these rows
carry object paths, and on a regulated bucket a path is a patient identifier,
so "delete this ontology" has to visibly mean the paths go too, rather than
resting on a constraint nobody reads.

**The in-memory store did leak**, and that one was real: its `Delete` removed
the ontology and its access rows and left `objects` keyed by an identifier
nothing resolves any more. A fake that does not reproduce the real store's
contract protects nothing, which is why the test for this lives there.

## What the page shows, and in what order

Somebody opening an ontology asks three questions: what is it, what is in it,
and how do I take a piece. The page had seven stacked blocks — the card, the
path shapes, the renaming form, a filter, a coverage table, the extracts, the
ownership transfer — and answered the third in sixth position.

It now opens on two:

1. **The card** — name, description, what the data means, what the platform
   counted, whether the ontology still describes its source, and the scans it
   has been through.
2. Everything administrative, in a closed **Settings** drawer: the reading
   rule, the object explorer (which ADR-025 asks for and which is therefore
   moved, not removed) and the ownership transfer.

Coverage appears between the two only when it has something to report — a
five-row table whose every cell says "nothing to report" is a passing check
rendered as an inventory.

**Extracts are not on this page.** The declaration form was here, which put a
cutting workshop on the page of the object that *describes*; an extract has
its own catalogue entry, and that is where one is declared, with the ontology
as the form's first field exactly as a dataset is picked before scanning. What
stays here is a fact about the ontology rather than an action on it: how many
extracts depend on it, which is what deleting or rescanning one engages.

### Figures that must agree with each other

Three of them did not, and each was found by reading one page end to end.

**The card's object count is what the reading produced**, not what the listing
walked past. SELENA displayed 3 994 at the top while the same page admitted,
at the bottom, one shape that had produced nothing — and the database held
3 993 rows. Keys the rule does not cover are stated separately: present in the
source, absent from extracts.

**The freshness check counts what the scan counts.** It counted every key,
directory markers included, while the scan deliberately skips them, so the two
figures could never agree. SELENA listed 4 111 against a manifest of 3 994 and
declared "no longer describes its source" the instant it was created, by
exactly its 117 directory keys; PREMYOM1000 was permanently stale by 3. A
warning that is always on is a warning nobody reads on the day the study
really does recruit eleven subjects.

**The shapes list is complete.** It kept the eight commonest and said nothing
about the rest: on SELENA those eight accounted for 3 979 objects out of 3 994
and the fifteen others appeared nowhere. The server returns them all and the
screen offers the tail — a list that looks exhaustive and is not is worse than
a short one.

### The study's name is read, not guessed

It was the first subject identifier with everything after the last dash
removed. `PREMYOM1000-0001` gave `PREMYOM1000`, which was right by luck, so
the rule shipped. `SELENA-01-001` gives `SELENA-01` — the investigating
centre, not the study — and the catalogue displayed a centre code as the name
of a trial, on every screen and on every extract cut from it.

The name is in the path already: it is the directory the subjects sit in. A
bucket whose subjects sit at the root names no study, and the dataset's own
name is used rather than inventing one. A rescan keeps the name somebody may
have corrected (ADR-043), so only new ontologies take the derived one.

## Every scan is kept

A rescan replaces the manifest and keeps the object (ADR-043). That is right
for the object — an ontology is what the source looks like *now* — and it left
two questions unanswerable five minutes later.

A subject count that moved from 2 to 31 is either the study recruiting or the
reading rule changing, and those call for opposite reactions. The platform
says which at the moment of the rescan, then throws away what it compared
against. And an extract freezes a file list and records the card version it
was cut against, while the manifest it was cut *from* is overwritten the next
time anybody presses scan — so "why does this cohort have 18 patients when the
ontology says 31" has no answer in the data.

So every scan is appended to `ontology_scans` and the ontology keeps pointing
at the newest. `GET /api/v1/ontologies/{id}/scans` returns them newest first;
each entry carries the counts, **the rule that produced them**, and the
difference against the scan before it. The oldest row carries no difference —
inventing a baseline of zero would report a first scan as a study that gained
thirty-one subjects overnight, which is the false alarm the feature exists to
remove.

The rule travels with the row because without it two photographs cannot be
told apart, and the whole point is telling them apart.

**What it costs.** A manifest is an aggregate — subjects, visits, modalities,
counts — not a file list, so a history is bounded by the shape of the study
rather than by its size; the per-file rows stay in `ontology_objects` and only
the current scan has them. Fifty photographs are kept per ontology, pruned on
insert: a dataset rescanned by a cron keeps a row per run, and an unbounded
table of manifests is the kind of slow leak only ever found by a disk alert.

This is the groundwork for versioning an ontology object: the manifests are
stored whole, so "read this ontology as it stood in March" is a query rather
than a migration.

## The three levels are named by the trade that owns the data

The positions were configurable and the words were not. So a platform also sold
to banks and insurers showed **subject**, **visit** and **modality** over a
table of transactions — which is not a purity problem but a demo that loses the
room, because the product visibly belongs to somebody else's business.

A dataset's layout (ADR-045) now carries three optional names beside its three
levels:

```json
{ "subjectLevel": 1, "visitLevel": 2, "modalityLevel": 3,
  "subjectName": "patient", "visitName": "visite", "modalityName": "modalité" }
```

```json
{ "subjectLevel": 0, "visitLevel": 1,
  "subjectName": "client", "visitName": "mois" }
```

**This is adaptation to a trade, not configuration of a mechanism.** Three words
somebody writes once — or that the assistant proposes from the path shapes it is
already shown, which a person then confirms, exactly as it proposes the rule
itself (ADR-040). Nothing else moves: the levels keep their positions, the
columns keep their names, and only what a human reads follows the trade.

Unnamed levels fall back to **entity / period / category** rather than to the
clinical words. A platform that must guess should guess neutrally.

Those fallbacks belong to the **screen**, never to the manifest.
`describeReading` used to send them, so a French page's own localised
fallbacks were never reached: one page displayed `entity`, `entitys`,
`Entités`, `Sujet` and `subject` for a single concept, the plural being an `s`
glued onto an English word nobody had chosen. A name travels in the manifest
only when somebody declared it; the rest is a label, and a label belongs to
whoever renders it, in whatever language it is read in.

The names are **recorded in the ontology's manifest** with the rule, for the
reason the rule is recorded: a name changed afterwards would make an old
photograph describe itself in a vocabulary it was never read with.

### Where they are filled in

Three fields beside the three levels, on the dataset's layout form. Or the
assistant proposes them: it is already shown the path shapes and asked which
level holds the grouping, the time and the kind, and it now proposes what the
trade calls them in the same answer — `names: patient / visit / modality`. It
proposes, a person confirms, and a word nobody confirmed is worse than the
generic default.

### Where they are read

Everywhere a screen names a level: the layout description, the list's Reading
column, the completeness and recognised-shapes panels, the query panel, the
extract form, the scan result and the rescan difference, and the card's
figures.

The prose that explains the platform itself is trade-neutral rather than
parameterised — "what groups the objects, when, and of what kind" instead of
"subjects, visits and modalities". A sentence about how the platform works
should not assume whose data it is.

What is deliberately **not** renamed: the Go fields, the JSON keys and the
database columns. `subject_id` appears in 43 statements, `Subject` in 53 Go
files; renaming them would be a data migration that breaks every stored
manifest, to change words no user reads.
