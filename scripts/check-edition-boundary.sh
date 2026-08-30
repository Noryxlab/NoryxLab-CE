#!/usr/bin/env bash
#
# Enforces the CE/EE boundary described in backend/ee/README.md and ADR-028.
#
# The boundary exists because NoryxLab-CE is public under MPL-2.0, whose
# copyleft is file-scoped: an Enterprise feature implemented inside an MPL file
# is an Enterprise feature given away. It closed once already, when the whole
# Enterprise surface lived in Community files behind NORYX_ENABLED_FEATURES,
# and it will close again under deadline pressure unless something fails loudly.
#
# Run from the repository root, or via `make check-edition`.
set -euo pipefail

cd "$(dirname "$0")/.."
BACKEND=backend
failures=0

fail() { printf '  FAIL  %s\n' "$1" >&2; failures=$((failures + 1)); }
pass() { printf '  ok    %s\n' "$1"; }

echo "CE/EE boundary"

# 1. Every ee_* file must be excluded from the Community build. A file without
#    the tag is linked into the public binary, which is the failure this whole
#    arrangement prevents.
missing_tag=0
while IFS= read -r file; do
  case "$(basename "$file")" in
    *_stub.go) continue ;;  # stubs are the Community side and are MPL
  esac
  if ! head -1 "$file" | grep -q '^//go:build enterprise$'; then
    fail "$file does not carry //go:build enterprise"
    missing_tag=1
  fi
done < <(find "$BACKEND" -name 'ee_*.go' -not -name '*_stub.go')
[ "$missing_tag" -eq 0 ] && pass "every ee_* file carries the enterprise tag"

# 2. Every ee_* file must carry the proprietary notice, because MPL applies to
#    any file that does not say otherwise.
missing_notice=0
while IFS= read -r file; do
  case "$(basename "$file")" in
    *_stub.go) continue ;;
  esac
  if ! head -6 "$file" | grep -q 'Enterprise Edition'; then
    fail "$file does not carry the proprietary notice"
    missing_notice=1
  fi
done < <(find "$BACKEND" -name 'ee_*.go' -not -name '*_stub.go')
[ "$missing_notice" -eq 0 ] && pass "every ee_* file carries the proprietary notice"

# 3. No MPL file may name an Enterprise feature constant. Community code asks
#    "is this capability available" through an extension point; it never
#    reasons about which Enterprise feature is licensed.
# The list is derived from the declarations rather than guessed, so a feature
# added later is covered without editing this script. Files carrying the
# enterprise tag are exempt whatever they are named: the tag, not the filename,
# is what keeps them out of the Community binary.
constants=$(grep -oE '^\tFeature[A-Za-z]+' "$BACKEND/internal/edition/hooks.go" | tr -d '\t' | paste -sd '|' -)
leaked=""
while IFS= read -r file; do
  head -1 "$file" | grep -q '^//go:build enterprise$' && continue
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

# 4. The Community build must compile and pass its tests without the tag. If it
#    does not, an Enterprise symbol has leaked into the core.
if go -C "$BACKEND" build ./... >/dev/null 2>&1; then
  pass "the Community edition builds without the tag"
else
  fail "the Community edition does not build: an Enterprise symbol leaked into the core"
fi

if go -C "$BACKEND" build -tags enterprise ./... >/dev/null 2>&1; then
  pass "the Enterprise edition builds with the tag"
else
  fail "the Enterprise edition does not build"
fi

# 5. The Community binary must contain no Enterprise route. This is the check
#    that would have caught the original problem: a feature reachable in the
#    public build.
for route in 'admin/backups' 'assistant/developer' 'admin/egress'; do
  if go -C "$BACKEND" vet ./internal/http/ >/dev/null 2>&1 &&
     grep -rq "$route" "$BACKEND/internal/http/server.go" 2>/dev/null; then
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
