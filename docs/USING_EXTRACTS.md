# Using extracts

A guide for people who have an ontology and want to work on the data. The
ontology side is in [USING_ONTOLOGIES.md](USING_ONTOLOGIES.md); the design
reasoning is in [ONTOLOGY.md](ONTOLOGY.md).

## What an extract is

**An extract is what you mount in a workspace.** The ontology says what is
there; the extract says what you take.

An ontology cannot be mounted, and that is deliberate: it describes a source,
it does not contain one. Every workload — workspace, app, job — sees extracts
and only extracts.

Two things make one:

| | |
|---|---|
| **The selection** | which files — by category, by kind of file, by entity, or everything |
| **The layout** | how the folders are arranged in the workspace |

The same files arranged two ways answer two different questions. "What does
this patient have" wants the entity first; "all my cornea scans, whoever they
belong to" wants the category first.

## It is frozen, and that is the point

An extract records its file list the moment you declare it. The same question
asked next month designates the same study, even if the bucket has grown and
the ontology has been rescanned since.

That is what makes a cohort citable. It also means an extract does **not**
follow the source: new files that arrive after it was declared are not in it,
and a rescan of the ontology underneath changes nothing about it. To include
what arrived since, declare a new extract.

## Declaring one

Catalogue → **Extracts** → *Declare an extract*. The ontology is the first
field, as a dataset is picked before scanning.

The form opens by saying where it is cutting from — the ontology's name, the
date that photograph was taken, and its figures. An extract freezes a list
taken from one reading at one moment, and without that line a file count is a
number with no provenance.

### 1 · What you take

Three ways to narrow, and they combine: **categories**, **kinds of file**, and
**entities**. Nothing ticked means everything.

Kinds of file is the one that makes a disclosure smaller than the folder it
lives in. On PREMYOM1000's ANTERION:

| kind | files | volume |
|---|---|---|
| DICOM images | 20 115 | 41 GB |
| CSV measurements | 349 | 37 MB |
| viewer chrome (PNG, GIF) | 140 | 7.7 MB |

Somebody who needs the measurements needs 37 MB. Before kinds were
selectable, the smallest thing they could ask for was the category — so they
were handed twenty thousand patient images to get three hundred CSVs. Ticking
`CSV` is the difference, and it is a governance difference, not a convenience
one.

It also lets the viewer chrome go: `IHE_PDI/images/arrowdwn.gif` is the
DICOM viewer's own decoration, not study data, and it is in every extract that
takes a whole ANTERION.

The kinds are counted at scan time, so an ontology scanned before this shows
no kinds at all — not "no files". Rescan it.

Entities are a comma-separated list.

The form says live what it will freeze — *"this extract will freeze 1,934 of
3,993 files"* — because an extract named `SELENA-modality` once left with an
empty filter, took the whole ontology, and nothing on screen contradicted its
name. A wrong n is discovered three months later, in a paper.

The three filters intersect, so with more than one set the figure is an upper
bound and says so.

**Mount the whole ontology** is the same thing with no filter at all, named so
the question "and if I want everything?" has an answer on screen.

### 2 · How you find it again

Folders are chosen one at a time, and each dropdown carries a real value from
this ontology beside the word:

```
Folder 1  [ Entité — SELENA-01-001 ▾ ]
Folder 2  [ Période — 20260218     ▾ ]
Folder 3  [ Catégorie — ANTERION   ▾ ]

/extracts/<name>/SELENA-01-001/20260218/ANTERION/files…
```

**Any of the six orders.** Choosing a level that is already placed swaps the
two — that is what reordering means.

**You can stop before three.** On a study with one visit per patient the visit
folder holds one entry per patient and exists only to be walked through;
`patient / modalité` is the tree you actually want.

Dropping a level merges what it separated, and the mount builds links — two
files on one path means one of them is not there. So the platform checks the
omission **against your actual selection** and refuses when it would lose a
file, naming the two files and the level to add back. Harmless on a study with
one visit per patient, destructive on the next study along: only the files can
say which, so only the files are asked.

## Mounting it

Attach the extract to a project; every workload that project starts sees it
under **`/extracts`** — a sibling of `/datasets` and `/repos`, not a directory
inside the project's own volume.

```
/extracts/selena-modality/anterion/selena-01-001/20260218/DICOM/…
```

Three things about that tree:

**They are links, never copies.** The bytes stay in the dataset mount, which
is read-only. A 400 GB study is not duplicated per workspace, and the platform
writes nothing to a regulated bucket.

**Names are lowercased and cleaned.** `SELENA-01-001` becomes
`selena-01-001`. It is the same rule every platform-built path follows.

**The tree is rebuilt at every start.** It is emptied first, so changing an
extract's layout and restarting gives you the new tree and not both at once.

The startup log says what happened:

```
[bootstrap] building extract links
[bootstrap] extract links ready: 3993 file(s)
```

## Why an extract might not be there

**Its dataset is not attached to the project.** An extract is a tree of links
*into* the dataset mount; without the dataset there is nothing for the links
to point at, so the platform mounts nothing rather than building a directory
of broken files. The startup log names it:

```
[bootstrap] extract not mounted - selena-modality: its dataset "HDS-For-selena"
            is not attached to this project, so there is nothing for its links
            to point at
```

Attach the dataset to the same project and restart.

**The file list is too large to ship.** The list travels in a Kubernetes
secret, capped at a megabyte. A very large extract is refused out loud rather
than mounted as a partial tree, because a tree missing files is a different
study and looks like a complete one.

**The ontology was scanned before file paths were kept.** Declaring the
extract refuses and tells you to rescan: an extract of zero files would look
like a legitimate empty result.

## Figures that should agree

A well-behaved import lines up like this, and the numbers are worth checking
against each other:

| Where | SELENA |
|---|---|
| Card: files | 3 993 |
| Card: outside the reading rule | 1 |
| Extract of everything | 3 993 |
| Links in the workspace | 3 993 |
| Broken links (`find /extracts -xtype l`) | 0 |

The file outside the reading rule — a stray `checksums/` here — is in the
bucket and in no extract. That is correct, and it is why the card states it
separately.

Zero-byte **directory keys** are excluded too. Some tools write a key for each
directory; linking one points straight into the read-only dataset and every
file beneath it is then skipped. On EMSE on 2026-10-01 a 4,019-file extract
mounted 186 files for exactly that reason.

## Living with one

- **Rename** it, transfer its **ownership**, attach it to **several projects**
  — the same gestures as a dataset or an ontology. Renaming changes what is
  displayed and the directory it mounts under.
- **Deleting** it removes the frozen list and nothing else: the source bucket
  is untouched, and so is the ontology.
- **Rescanning the ontology** underneath does not change it. That is the
  freezing working as intended.

## What the platform never does

- It never copies your data. An extract is links.
- It never writes to the source bucket.
- It never mounts an ontology — only an extract, so what a workload sees is
  always something somebody declared on purpose.
