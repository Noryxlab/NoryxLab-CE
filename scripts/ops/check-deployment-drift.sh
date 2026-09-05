#!/usr/bin/env bash
#
# Says what a deployment is about to lose.
#
#   ./scripts/ops/check-deployment-drift.sh <namespace> <deployment> <rendered-manifest> [VAR ...]
#
# Applying a manifest replaces the pod template wholesale. Anything an operator
# set by hand on the live deployment - an environment variable, an image
# pinned during an incident - is gone at the next release, silently, and the
# platform keeps answering so nobody looks.
#
# This compares the live deployment with the manifest about to be applied, plus
# the variables the caller says it will set afterwards, and refuses when a value
# would disappear. It cost three interruptions in one day to decide this was
# worth twenty lines:
#
#   - NORYX_PROJECT_FILES_IMAGE, left at a dev build for months because the
#     manifest carried a placeholder nobody overrode;
#   - the backend image reset to that same placeholder mid-deployment;
#   - PGDATA naming a directory no installation used, which sent a recreated
#     Postgres into initdb beside the real cluster.
#
# Exit 1 on a loss, unless ALLOW_ENV_LOSS=1 - which an operator sets knowingly,
# on a line they had to type.
set -euo pipefail

NAMESPACE="${1:-}"
DEPLOYMENT="${2:-}"
MANIFEST="${3:-}"
shift 3 2>/dev/null || true
WILL_SET="$*"

if [ -z "$NAMESPACE" ] || [ -z "$DEPLOYMENT" ] || [ -z "$MANIFEST" ]; then
  echo "usage: $0 <namespace> <deployment> <rendered-manifest> [VAR ...]" >&2
  exit 2
fi

# A deployment that does not exist yet cannot lose anything.
if ! kubectl -n "$NAMESPACE" get deployment "$DEPLOYMENT" >/dev/null 2>&1; then
  echo "  drift: $DEPLOYMENT does not exist yet in $NAMESPACE, nothing to lose"
  exit 0
fi

live="$(kubectl -n "$NAMESPACE" get deployment "$DEPLOYMENT" -o json)"

printf '%s' "$live" | python3 -c '
import json, sys, re

live = json.load(sys.stdin)
manifest_path, deployment, will_set = sys.argv[1], sys.argv[2], set(sys.argv[3].split())

container = live["spec"]["template"]["spec"]["containers"][0]
live_env = {entry["name"]: entry.get("value", "<from a secret>") for entry in container.get("env", [])}
live_image = container.get("image", "")

manifest = open(manifest_path).read()
# The manifest is read as text on purpose: it may hold several documents and
# the point is only which names appear in it, not a faithful parse.
manifest_env = set(re.findall(r"- name: ([A-Z][A-Z0-9_]+)", manifest))
manifest_image = ""
image_match = re.search(r"^\s+image: (\S+)", manifest, re.M)
if image_match:
    manifest_image = image_match.group(1)

lost = sorted(name for name in live_env if name not in manifest_env and name not in will_set)
if live_image and manifest_image and live_image != manifest_image:
    print(f"  drift: image {live_image} -> {manifest_image}")
    print(f"         (expected when the deployment sets it afterwards)")

if not lost:
    print(f"  drift: {deployment} keeps every variable it has")
    sys.exit(0)

print(f"  drift: {deployment} would lose {len(lost)} variable(s) set outside the manifest:")
for name in lost:
    value = live_env[name]
    if len(value) > 60:
        value = value[:57] + "..."
    print(f"         {name}={value}")
sys.exit(1)
' "$MANIFEST" "$DEPLOYMENT" "$WILL_SET" && exit 0 || status=$?

if [ "${ALLOW_ENV_LOSS:-0}" = "1" ]; then
  echo "  drift: continuing anyway, ALLOW_ENV_LOSS=1"
  exit 0
fi
echo "  Set ALLOW_ENV_LOSS=1 to apply regardless, or add these to the manifest." >&2
exit "$status"
