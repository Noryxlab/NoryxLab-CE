# The Noryx API

Two documents describe it, generated from the router by
`scripts/ops/generate-openapi.py` and checked by CI:

| Document | Served at | What it holds |
| --- | --- | --- |
| `openapi.public.yaml` | `/swagger/openapi.public.yaml` | the supported API: what an integration may build on |
| `openapi.yaml` | `/swagger/openapi.yaml` | everything the platform serves, internal operations included |

Neither count is quoted here. They were, and they rotted: the prose said 168
and 187 while the documents held 236 and 257, because CI checks the documents
against the router and nothing checks a sentence against the documents. The
documents are the count.

Swagger UI is at `/swagger`, which loads the supported document; `/swagger?spec=full`
loads the other, and each page links to the other. It is
served by the platform - no request leaves the installation to render it.

## Supported and internal

An endpoint is **internal** when only our own interface or our operators call
it, and the shape of its answer follows a screen rather than a domain object:
the console's dashboards and aggregations, the search box, a user's interface
preferences, the workload proxies. Those carry `x-noryx-internal: true` and are
absent from the supported document.

They are still documented, because someone reading the platform's own network
traffic should be able to find out what a call is. They carry no compatibility
promise: they change with the screens that use them.

Everything else is a contract. It is versioned under `/api/v1`, every operation
says what it returns, and a test fails the build when one does not - including
the ones added tomorrow.

## Adding an endpoint

1. Register it in the router.
2. Run `python3 scripts/ops/generate-openapi.py`. It writes the entry, and both
   documents.
3. Give it a real summary, and say what it returns. If it returns an envelope
   the generator cannot read from a Go struct, declare it in `ENVELOPES` and
   `RESPONSES` in the generator - that is what keeps the description and the
   code from drifting apart.
4. If only the interface will ever call it, add its path to `INTERNAL` in the
   generator instead.

CI runs `generate-openapi.py --check`, which fails when a route is
undocumented **and** when either document is out of date with the source.

## The API is list-shaped

There is no "read one object" endpoint for a dataset, an ontology, an extract,
a data source, a repository, a project, an app, a job, a workspace, a build or
an environment. Only a secret has one. Every screen that shows one object
lists them and picks, which is why nothing in the interface misses it.

An integration should expect the same: ask for the collection, filter on the
`id`. It also means the collection is the access decision - an object you hold
no right on is absent from your listing rather than refused by name, which is
the same answer `GET` on a named object gives elsewhere: not found, never
forbidden, because naming an identifier should not confirm that it exists.

Measured on 2026-10-30 by a sweep that generated two accounts and used nothing
but this API.
