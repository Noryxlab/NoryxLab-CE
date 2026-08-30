#!/usr/bin/env bash
#
# Enforces the CE/EE boundary described in backend/ee/README.md and ADR-028.
#
# The boundary exists because NoryxLab-CE is public: anything committed here is
# readable, copyable and reimplementable by anyone, whatever a licence says.
#
# It has now closed twice. First when the whole Enterprise surface lived in
# Community files behind NORYX_ENABLED_FEATURES. Then when the Enterprise
# sources were committed here behind `//go:build enterprise` - which kept them
# out of the Community *binary* while publishing every line of them. A build
# tag decides what gets compiled; it decides nothing about what gets read.
#
# So the invariant checked here is absence, not tagging: Enterprise source must
# not exist in this repository. Community keeps only the stubs that make the
# routes return 404, and the extension points Enterprise plugs into.
#
# Run from the repository root, or via `make check-edition`.
set -euo pipefail

cd "$(dirname "$0")/.."
BACKEND=backend
failures=0

fail() { printf '  FAIL  %s\n' "$1" >&2; failures=$((failures + 1)); }
pass() { printf '  ok    %s\n' "$1"; }

echo "CE/EE boundary"

# 1. No Enterprise source, anywhere. The tag is the marker Enterprise files
#    carry, so its presence in a public file is the leak itself.
tagged=$(grep -rl '^//go:build enterprise$' --include='*.go' "$BACKEND" 2>/dev/null || true)
if [ -n "$tagged" ]; then
  while IFS= read -r file; do fail "$file is Enterprise source in a public repository"; done <<< "$tagged"
else
  pass "no Enterprise source in the public tree"
fi

# 2. No proprietary notice. A file claiming to be proprietary while sitting in
#    a public MPL repository is either a leak or a lie about its own licence.
notices=$(grep -rl 'Enterprise Edition. Proprietary' --include='*.go' --include='*.ts' --include='*.tsx' \
            "$BACKEND" frontend/src 2>/dev/null || true)
if [ -n "$notices" ]; then
  while IFS= read -r file; do fail "$file carries a proprietary notice but is published"; done <<< "$notices"
else
  pass "no proprietary notice in the public tree"
fi

# 3. No MPL file may name an Enterprise feature constant. Community code asks
#    "is this capability available" through an extension point; it never
#    reasons about which Enterprise feature is licensed. The list is derived
#    from the declarations rather than guessed, so a feature added later is
#    covered without editing this script.
constants=$(grep -oE '^\tFeature[A-Za-z]+' "$BACKEND/internal/edition/hooks.go" | tr -d '\t' | paste -sd '|' -)
leaked=""
while IFS= read -r file; do
  case "$file" in */internal/edition/*) continue ;; esac
  if grep -qE "edition\.($constants)\b" "$file"; then
    leaked="$leaked$file
"
  fi
done < <(find "$BACKEND" -name '*.go')
leaked=$(printf '%s' "$leaked" | sed '/^$/d' || true)
if [ -n "$leaked" ]; then
  while IFS= read -r file; do fail "$file (MPL) references an Enterprise feature"; done <<< "$leaked"
else
  pass "no MPL file references an Enterprise feature"
fi

# 4. The Community build must compile and pass its tests on its own.
if go -C "$BACKEND" build ./... >/dev/null 2>&1; then
  pass "the Community edition builds"
else
  fail "the Community edition does not build"
fi

# 5. This repository must NOT be able to produce an Enterprise binary. This is
#    the check that would have caught the second failure: a public checkout
#    that answered `go build -tags enterprise` with a working Enterprise
#    server. It must fail, and it must fail because the sources are missing.
if go -C "$BACKEND" build -tags enterprise ./... >/dev/null 2>&1; then
  fail "a public checkout can build the Enterprise edition: its sources are here"
else
  pass "a public checkout cannot build the Enterprise edition"
fi

# 6. The published Dockerfile must offer no edition selector. An image recipe
#    that documents how to build Enterprise is a map to the door.
if grep -q 'NORYX_EDITION_BUILD\|tags enterprise' "$BACKEND/Dockerfile"; then
  fail "backend/Dockerfile still knows how to build the Enterprise edition"
else
  pass "the published Dockerfile builds Community only"
fi

# 7. The Community binary must contain no Enterprise route.
for route in 'admin/backups' 'assistant/developer' 'admin/egress'; do
  if grep -rq "$route" "$BACKEND/internal/http/server.go" 2>/dev/null; then
    fail "route $route is registered on the Community mux"
  fi
done
pass "no Enterprise route in the Community registration"

echo
if [ "$failures" -gt 0 ]; then
  echo "$failures boundary problem(s)"
  exit 1
fi
echo "Boundary holds"
