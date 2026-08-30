# The Enterprise boundary

This directory documents a boundary; it holds no Enterprise code, and it must
never hold any.

## Why the code is not here

NoryxLab-CE is public. Anything committed here can be read, copied and
reimplemented by anyone, whatever a licence file says. A licence governs use
and redistribution; it does not govern reading.

That distinction was learned twice. The first arrangement put the whole
Enterprise surface in Community files behind `NORYX_ENABLED_FEATURES`, so an
environment variable unlocked paid features. The second put the Enterprise
sources in this repository behind `//go:build enterprise`, which fixed the
binary and not the leak: the Community image no longer contained the machine
code, but every line of the source was still published, and a public checkout
answered `go build -tags enterprise` with a working Enterprise server.

A build tag decides what gets compiled. It decides nothing about what gets
read. So the boundary is now physical: Enterprise source lives in
`NoryxLab-EE`, and this repository cannot produce an Enterprise binary because
the files are absent.

## What Community keeps

- **Stubs** (`ee_*_stub.go`), which are MPL and are the Community behaviour.
  They make Enterprise routes return **404, not 403**: a Community deployment
  should not advertise the existence of doors it has no key to.
- **Extension points** (`internal/edition`, `frontend/src/lib/extensions.ts`),
  which are the declared contract Enterprise plugs into.

Community code asks whether a capability is available. It never reasons about
which Enterprise feature is licensed, and no MPL file names an Enterprise
feature constant — `scripts/check-edition-boundary.sh` fails the build if one
does.

## How Enterprise is built

From `NoryxLab-EE`, which carries `overlay/` — the Enterprise sources, laid out
at the paths they occupy in a Community checkout. The private build copies the
overlay over a checkout of this repository and compiles with the tag. Same
lineage, no fork, and nothing published.

## Adding an Enterprise feature

Put the code in `NoryxLab-EE/overlay/`. If it needs something from Community,
add an extension point here and use it from there. If you find yourself wanting
to add `//go:build enterprise` to a file in this repository, the boundary is
closing again — that is precisely the shape of the second failure.
