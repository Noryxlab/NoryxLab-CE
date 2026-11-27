# Training attestations

Each person can upload a PDF saying they were trained to handle confidential
data. The platform records it. **It refuses nothing.**

## What it does, and what it deliberately does not

It records a document, who uploaded it, what they say it covers, and when it
stops being valid. It shows an administrator who has produced one.

It does **not** gate anything: not attaching a regulated dataset, not launching
a workspace beside one, not opening an application. Collecting before enforcing
is the order that works - an installation that starts requiring a field nobody
has filled locks everybody out on the first day, while one that has been
collecting for months can see who is covered before it decides.

It is also not a certificate in the cryptographic sense. It is the PDF somebody
was handed at the end of a course, scanned or exported: no signature to check,
no chain to validate. The platform does not read it.

## The expiry is the point

The date is optional and it is what makes the record useful. Without one, a
record answers "did they ever do the training". With one, it answers "are they
covered today", which is the question that gets asked.

Three statuses, computed from the date and never stored - a document does not
expire by being written to:

| | |
| --- | --- |
| **valid** | a date, still ahead |
| **expired** | a date, now passed |
| **undated** | nobody said when it runs out — **not** "valid forever" |

A date covers the whole day it names: a certificate valid until 31 March is
valid on 31 March.

## Who can read a document

Its owner, and a platform administrator. Nobody else — not colleagues, not a
project administrator. These documents carry a name, an employer and sometimes
a date of birth, and being on somebody's team says nothing about the right to
read that. Asking for somebody else's attestation answers **not found** rather
than forbidden, because a refusal would confirm the identifier exists.

Uploading, downloading and deleting are each audited.

## Where it lives

The person uploads from **Account**; an administrator reads the list under
**Administration → Identity**.

The PDF goes to a bucket of the platform's own, never a dataset's, keyed by the
person. At most 10 MiB, and the file is checked on its first bytes rather than
on its name — a Word export renamed `.pdf` is the ordinary way this goes wrong,
and the person who would otherwise find out is an auditor opening it a year
later.

Uploads accumulate rather than replace. A re-certification is a new document,
and overwriting the old one would lose the answer to "were they covered in
March", which is what an audit asks.

## If you later want to require it

Nothing here needs changing to enforce: the record and its status already
exist. What needs deciding first is what happens to somebody without one on
the day it starts being required, and that is a question for the installation
rather than for the platform.
