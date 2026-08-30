# Enterprise Edition sources

Files named `ee_*.go` anywhere in this backend are **not** covered by the
MPL-2.0 licence at the repository root. They carry the proprietary notice in
`ee/LICENSE` and each file repeats it in its header.

This is the arrangement MPL-2.0 is designed for. Its copyleft is file-scoped:
MPL files stay MPL, and separate files may be combined with them in a larger
work under different terms (see `ADR-028`). The Enterprise sources are separate
files.

## Two problems, two mechanisms

**Licensing.** A proprietary header on each file. Being readable in a public
repository is not the same as being usable: MPL grants nothing over files it
does not cover, and these files grant nothing without a subscription
agreement.

**Distribution.** Every `ee_*.go` file carries `//go:build enterprise`. The
Community binary is compiled without that tag, so the Enterprise code is not
merely disabled in it — **it is not in it**. No environment variable, no
feature flag and no configuration can enable a feature whose machine code was
never linked.

That distinction is the point. `NORYX_ENABLED_FEATURES` used to switch
behaviour on inside a single binary that shipped to everyone, which meant the
only thing standing between a Community deployment and the Enterprise features
was an operator's willingness to set a variable.

## Building

```sh
go build ./cmd/noryx-api                     # Community
go build -tags enterprise ./cmd/noryx-api    # Enterprise
```

The container image selects an edition through the `NORYX_EDITION_BUILD` build
argument.

## Rules

- An Enterprise feature lives in an `ee_*.go` file. No exceptions: a feature
  half-implemented in an MPL file is a feature given away.
- Where Community code needs to call into an Enterprise capability, the
  Community side declares an extension point with a no-op default in an
  `ee_*_stub.go` file guarded by `//go:build !enterprise`. The stub is MPL and
  does nothing; the implementation is proprietary.
- `scripts/check-edition-boundary.sh` fails the build if an MPL file references
  an Enterprise-only symbol.
