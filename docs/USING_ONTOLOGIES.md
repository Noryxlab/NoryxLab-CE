# Using ontologies

A guide for people who open the catalogue and want to get work done. The
design reasoning and the storage model are in [ONTOLOGY.md](ONTOLOGY.md); this
page is about the screens.

## What an ontology is

**An ontology is how the platform reads a bucket.** It is not a copy of your
data and it does not contain anything: it is a photograph of what is in the
source and how it is arranged, taken at one moment, by one rule.

Three consequences follow, and they explain most of what the screens do:

- It can be **out of date**, because the source keeps living after the
  photograph was taken.
- It can be **wrong**, if the rule that read the paths was wrong — nothing in
  the data tells the platform which directory holds a patient.
- It cannot be **mounted**. What you mount in a workspace is always an
  *extract*: the ontology says what is there, the extract says what you take.

## The journey, once

```
Analyse a source  →  check the reading  →  describe what it means  →  cut an extract
```

Scanning is the only step that touches the bucket, and it only ever **lists**
object names. The platform never reads inside your files unless you explicitly
run a structure scan, and it never writes.

## 1. Analyse a source

Catalogue → Ontologies → *Analyse a source*. Pick a dataset; the scan walks
the object keys and builds the photograph.

Scanning the **same source twice refreshes the same ontology** rather than
standing a duplicate beside it. The new picture replaces the old one and
everything else is kept: the name you may have corrected, the owner, the
permissions, the description, and the extracts already cut from it — whose
file lists are frozen and therefore unaffected.

One source has exactly one ontology. If somebody else already described this
bucket and you cannot see their ontology, the scan refuses and tells you to
ask for access rather than creating a second description of the same thing.

## 2. Check the reading

This is the step people skip, and the only one that can silently ruin
everything downstream.

The platform reads a path by **position**: which level holds the grouping,
which holds the time, which holds the kind. Counted from zero:

```
SELENA / SELENA-01-001 / 20260218 / ANTERION / DICOM / f.dcm
  0           1              2          3
            entity         period    category
```

The **Reading** column in the list says which rule produced each row:

| It says | It means |
|---|---|
| **Declared** | Somebody who knows the study confirmed the rule. |
| **Default** | The platform guessed, using a compiled rule that looks for a segment resembling an identifier. |
| **Not recorded** | This photograph predates the rule being recorded. How it was read is unknown; rescan to find out. |

A row's figures are worth exactly what its reading is worth. A default reading
is not wrong by nature — it is right on most studies — but nobody has
confirmed it.

### Declaring the reading

Settings → *How the source is read*. Three numbers, and three words.

You can write them yourself, or ask the assistant. *Ask for a proposal* sends
it the **path shapes** — never the paths themselves, because on a regulated
bucket a key is a patient identifier. It answers with a line like:

```
subject: level 1, visit: level 2, modality: level 3
names: patient / visite / modalité
```

Paste its whole answer into *Paste the assistant's proposal*. The six fields
fill, and the rule is immediately tried on twelve real paths so you can see
what it reads out of each. **Pasting stores nothing** — you check, then press
*Store the rule*, then scan again for it to apply.

The trial is the check worth doing: a level off by one shows up there in one
glance, and after a scan it shows up as a subject count nobody can explain.

### The three words

The levels keep their positions; the words are what humans see on screen.
Write `patient / visite / modalité` on a clinical study, `client / mois /
opération` on a ledger. Leave them empty and the platform stays generic —
*entity*, *period*, *category* — rather than talking clinical to a bank.

A level the source does not carry is left **empty**, which is truer than
pointing at a segment that means something else. A study with one visit per
patient simply has no visit level.

## 3. Read the card

### What the ontology describes

One free text field, written by a person. The scan counts what it finds; what
the data *means* — what it may be used for, what it must not be used for,
under what right it is held, who to ask — is not inferable from any number of
bytes.

It is versioned. Every edit increments the version and records who wrote it,
so a cohort cut against version 3 can still say so after version 4 is written.

### What the platform measured

Beside the declaration, never instead of it. Trust does not exclude
verification, and these figures are the platform's own:

- **Entities / files / categories / period** — what the reading produced.
- **"n files outside the reading rule"** — keys the rule does not cover. They
  exist in the source and will **not** appear in any extract. One file here is
  usually a stray `checksums/` or a README; a hundred means the rule is wrong.
- **"File contents never checked"** — nobody has looked *inside* the files.
  This is not "nothing was found": it is "nobody looked". Counting objects
  says nothing about what they contain, and in particular nothing about
  whether they carry patient identifiers.

### Freshness

A banner appears when the source no longer holds what the photograph says.
It compares object counts, ignoring the directory markers that some tools
leave behind, and tells you how far the source has drifted. Everything built
on a stale ontology silently omits whatever arrived since — an extract above
all.

## 4. The scan history

Every photograph is kept, newest first, with **the rule that produced it** and
what changed against the one before.

That column is the point. A subject count that moves from 2 to 31 has two
possible causes — the study recruited, or somebody changed the reading — and
they call for opposite reactions. A row that says *Reading changed* next to
*+29 patients* has told you which.

The oldest row shows no difference, because there is nothing before it to
compare against.

Fifty scans are kept per ontology. A second scan of an unchanged source says
*nothing moved*, which is the useful case: it is the proof that the scan is
reproducible.

## 5. Coverage, when there is a gap

A panel appears **only when something is missing**: an entity that does not
carry a category the others do. It matters because an extract requested by
category silently excludes the entities that lack it — you would get n=1 and
nothing on screen would say so.

No panel means no gap.

## 6. Explore the content

One row per entity × period × category, with the file count. It answers what
the other panels cannot:

- **A partial export.** Two patients at 975 and 959 files are comparable; 975
  and 12 is an interrupted transfer. The card would show the same total in
  both cases, and coverage would say "2/2, none missing" in both cases,
  because the category *is* present.
- **Whether the reading worked on the whole bucket**, not on the twelve paths
  the trial sampled.
- **What one particular entity holds.**

The filter box searches one word across the three columns. Empty shows
everything.

## 7. Cut an extract

Two things make an extract: **what you take**, and **how it is arranged**.

The selection — categories, entities — says which files. The layout says what
the folders look like in the workspace, and it is built one folder at a time:

```
Folder 1  [ patient  ▾ ]      SELENA-01-001 / 18 Feb 2026 / ANTERION / files…
Folder 2  [ visite   ▾ ]
Folder 3  [ modalité ▾ ]
```

Any order. "What does this patient have" and "all my cornea scans, whoever
they belong to" are different questions over the same files, and the second
one wants the category first.

You can also **stop before three**. On a study with a single visit per
patient, the visit folder holds one entry per patient and exists only to be
walked through; `patient / modalité` is the tree you actually want.

Dropping a level merges what it separated, and the mount builds a tree of
links — two files on one path means one of them is not there. So the platform
checks the omission **against your actual selection** and refuses when it
would lose a file, naming the two files and the level to add back. Harmless
on a study with one visit per patient, destructive on the next study along:
only the files can say which, so only the files are asked.

Not on this page: extracts have their own catalogue entry, and that is where
one is declared, with the ontology as the form's first field. The ontology
page only tells you **how many extracts depend on it**, which is what deleting
or rescanning one engages.

## Things that go wrong, and what they mean

**"The scan recognised nothing."** The reading rule does not fit this bucket.
Look at the recognised and unrecognised shapes under Settings, ask the
assistant for a proposal, try it, store it, scan again.

**An entity count that is far too high** — one per file, say. A level is
pointing at the file name instead of the directory above it. The trial would
have shown it: the entity column would hold file names.

**An entity count of 1** when you expect many. A level too shallow: everything
is being filed under the study directory. The trial would show the same value
in the entity column on every line.

**A category called `unknown`.** The modality level is absent or points at a
segment some paths do not have. That is legitimate when the source genuinely
has one instrument; otherwise the rule needs a level.

**Figures that moved after a rescan.** Read the history row. If it says
*Reading changed*, the data did not move — the rule did.

## What the platform never does

- It never writes to your bucket. Scanning lists object names, nothing else.
- It never sends paths, file names or identifiers to a model. The assistant is
  shown **shapes** — `6 levels · text/subject/date/text/text/DICOM (2001)` —
  which carry no identifier.
- The assistant applies nothing. It proposes; a person confirms; the platform
  stores.
- The scan itself involves no model at all. It is deterministic: the same
  bucket read by the same rule gives the same answer, every time, which is
  what makes "nothing moved" mean something.
