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

## Reading one object

`GET /api/v1/<collection>/{id}` answers with one dataset, ontology, extract,
data source, repository, project, app, job, workspace, build or secret.

Each answers the same question its collection answers, with the same rule, so
the two cannot disagree: an object absent from your listing is absent here.
The refusal is **not found, never forbidden** — naming an identifier must not
confirm that it exists, so an object somebody else owns and an identifier that
never existed are indistinguishable.

These were missing until 2026-10-30. The API was list-shaped: every screen
that shows one object asks for the collection and picks, which is why nothing
in the interface noticed, while an integration had to fetch every dataset to
look at one — and a `GET` on a named object answered 404 from the router,
which reads exactly like a refusal.

An **environment** has none, and that is not an omission: it is not a stored
object but a view computed over builds, keyed by the image its revisions push
to. Reading one means filtering `GET /api/v1/environments`, which is what the
screen does.
