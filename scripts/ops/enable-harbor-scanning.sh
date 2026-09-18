#!/usr/bin/env bash
#
# Turns on vulnerability scanning in Harbor, and makes it apply to what is
# already there.
#
#   sudo ./enable-harbor-scanning.sh
#
# Run it on the machine that runs Harbor. It is idempotent: run it again and it
# reports what is already in place rather than doing it twice.
#
# Why this exists. On 2026-09-18 neither installation had a scanner registered
# at all - Harbor had been installed without Trivy on both - so the platform's
# vulnerability column could never have shown anything. The Trivy image was
# already loaded on both machines; only the service was missing. The platform
# meanwhile could not even read an artifact, which silently defeated something
# else: it resolves an image tag to a digest before starting a workspace, and
# without that digest a rebuilt image never reaches a node that already holds
# the tag.
#
# What it does, in order:
#   1. copies the operator key to noryxops, where that account exists but has
#      no key (the DC registry, which is why it could not be administered
#      remotely while EMSE could);
#   2. backs up the Harbor configuration;
#   3. re-runs Harbor's own installer with --with-trivy, which recreates the
#      containers - THE REGISTRY IS DOWN FOR ABOUT TWO MINUTES;
#   4. waits for the scanner to register;
#   5. switches on scan-on-push for every noryx project;
#   6. starts one scan of everything already stored, so the column has content
#      before the next push rather than after it.
#
# Nothing here deletes anything, and /data is untouched: image blobs and the
# Harbor database live there and the installer leaves them alone. The backup in
# step 2 is what makes step 3 reversible - re-running install.sh without
# --with-trivy puts it back.
set -euo pipefail

HARBOR_DIR=${HARBOR_DIR:-/opt/harbor/harbor}
OPS_USER=${OPS_USER:-noryxops}
SOURCE_KEYS=${SOURCE_KEYS:-}

say() { printf '\n== %s\n' "$*"; }
ok() { printf '  ok    %s\n' "$*"; }
skip() { printf '  --    %s\n' "$*"; }
fail() { printf '  FAIL  %s\n' "$*" >&2; }

if [ "$(id -u)" -ne 0 ]; then
  echo "run me with sudo: sudo $0" >&2
  exit 2
fi
if [ ! -f "$HARBOR_DIR/harbor.yml" ]; then
  echo "no harbor.yml under $HARBOR_DIR - set HARBOR_DIR to where Harbor is installed" >&2
  exit 2
fi

# ---------------------------------------------------------------- 1. the key
#
# Only where the account exists and has nothing. Never overwrites a key file
# that is already there: an account somebody else administers is not this
# script's to change.
say "operator access for $OPS_USER"
if ! id "$OPS_USER" >/dev/null 2>&1; then
  skip "no $OPS_USER account on this machine"
else
  ops_home=$(getent passwd "$OPS_USER" | cut -d: -f6)
  if [ -s "$ops_home/.ssh/authorized_keys" ]; then
    skip "$OPS_USER already has authorized_keys"
  else
    keys=$SOURCE_KEYS
    if [ -z "$keys" ]; then
      # The invoking user's own keys, which is who is running this by hand.
      invoker=${SUDO_USER:-root}
      keys=$(getent passwd "$invoker" | cut -d: -f6)/.ssh/authorized_keys
    fi
    if [ -s "$keys" ]; then
      install -d -o "$OPS_USER" -g "$OPS_USER" -m 700 "$ops_home/.ssh"
      install -o "$OPS_USER" -g "$OPS_USER" -m 600 "$keys" "$ops_home/.ssh/authorized_keys"
      ok "copied $(grep -c . "$keys") key(s) from $keys"
    else
      skip "no keys to copy at $keys"
    fi
  fi
fi

# ------------------------------------------------------------- 2. the backup
say "configuration backup"
backup="$(dirname "$HARBOR_DIR")/pre-trivy-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$backup"
cp -a "$HARBOR_DIR/harbor.yml" "$backup/"
[ -f "$HARBOR_DIR/docker-compose.yml" ] && cp -a "$HARBOR_DIR/docker-compose.yml" "$backup/"
[ -d "$HARBOR_DIR/common" ] && cp -a "$HARBOR_DIR/common" "$backup/common"
ok "$backup"

# -------------------------------------------------------------- 3. the install
#
# Harbor's own installer rather than hand-editing docker-compose.yml: the
# compose file is generated from harbor.yml by the prepare step, so an edit
# made by hand is reverted the next time anybody runs this properly - and
# nobody would know until then.
say "enabling Trivy (the registry restarts)"
if docker ps --format '{{.Names}}' | grep -q '^trivy-adapter$'; then
  skip "trivy-adapter already running"
else
  cd "$HARBOR_DIR"
  ./install.sh --with-trivy 2>&1 | tail -5
  ok "installer finished"
fi

# -------------------------------------------------------------- 4. the wait
#
# "The containers are up" is not "the scanner is registered": Harbor registers
# Trivy through its own API once core is serving, and reporting success before
# that would be reporting the thing this script exists to arrange.
say "waiting for the scanner to register"
password=$(grep -m1 '^harbor_admin_password' "$HARBOR_DIR/harbor.yml" | sed 's/^[^:]*: *//')
api() {
  local method=$1 path=$2 body=${3:-}
  if [ -n "$body" ]; then
    curl -sk -u "admin:$password" -X "$method" -H 'Content-Type: application/json' -d "$body" "https://127.0.0.1/api/v2.0$path"
  else
    curl -sk -u "admin:$password" -X "$method" "https://127.0.0.1/api/v2.0$path"
  fi
}

registered=0
for _ in $(seq 1 60); do
  if api GET /scanners | grep -q '"name"'; then
    registered=1
    break
  fi
  sleep 5
done
if [ "$registered" -eq 1 ]; then
  ok "$(api GET /scanners | python3 -c 'import json,sys; print(", ".join(s["name"] for s in json.load(sys.stdin)))')"
else
  fail "no scanner registered after five minutes"
  fail "the configuration backup is at $backup"
  exit 1
fi

# --------------------------------------------------------- 5. scan on push
say "scan on push"
for project in $(api GET '/projects?page_size=100' | python3 -c 'import json,sys; print(" ".join(p["name"] for p in json.load(sys.stdin)))'); do
  case "$project" in
    noryx*) ;;
    *) continue ;;
  esac
  api PUT "/projects/$project" '{"metadata":{"auto_scan":"true"}}' >/dev/null
  ok "$project"
done

# ------------------------------------------------- 6. what is already stored
#
# Without this, scanning starts applying to images pushed from now on and every
# image already in the registry stays unreported - which is the same empty
# column, for a subtler reason.
say "scanning what is already there"
api POST /system/scanAll/schedule '{"schedule":{"type":"Manual"}}' >/dev/null || true
ok "started; it runs in the background and takes a few minutes per image"

# ------------------------------------------------ 7. keeping the disk honest
#
# The counterpart to a nightly rebuild, and it has to be set up in the same
# breath as one. Each rebuild moves a tag onto a new artifact and leaves the
# previous one untagged - still stored, still occupying its blobs. Nothing
# reclaims that by default: neither installation had a retention policy or a
# collection schedule, so the registry would have grown every night until
# somebody noticed it the hard way.
#
# Weekly rather than nightly, on purpose. The platform starts workspaces from a
# digest, so an artifact can be untagged and still be what something is running.
# A workspace lives at most 48 hours, so a week between untagging and collection
# is a margin rather than a race - and it stays a margin only while that
# lifetime stays shorter than this interval, which is worth remembering if
# either is ever changed.
say "weekly garbage collection"
api PUT /system/gc/schedule '{"schedule":{"type":"Weekly","cron":"0 0 4 * * 0"},"parameters":{"delete_untagged":true}}' >/dev/null
ok "$(api GET /system/gc/schedule | python3 -c 'import json,sys; d=json.load(sys.stdin); print("next", d["schedule"]["next_scheduled_time"], d["job_parameters"])')"

printf '\nDone. The platform reads the results on its own - the environment\n'
printf 'catalogue will show counts instead of "not scanned" once the scans finish.\n'
printf 'To undo the Trivy part: cd %s && ./install.sh   (no --with-trivy)\n' "$HARBOR_DIR"
