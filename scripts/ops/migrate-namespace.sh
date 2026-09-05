#!/usr/bin/env bash
#
# Moves a Noryx installation from one namespace to another, without copying the
# data.
#
#   ./scripts/ops/migrate-namespace.sh noryx-ce noryx
#
# Why this exists: the namespace was called `noryx-ce` on platforms running
# Enterprise, which reads as a contradiction every time an operator opens a
# shell. Kubernetes has no rename, so this is what a rename actually is.
#
# What it does *not* do is copy volumes. A PersistentVolume is cluster-scoped
# and already holds the data; it is the *claim* that belongs to a namespace. So
# each volume is set to Retain, released from its old claim, and bound to a new
# claim in the target namespace. Nothing is written, nothing is copied, and a
# 100 GiB volume moves as fast as a 1 GiB one.
#
# The two dangerous moments are named here rather than hidden:
#
#   1. Between deleting the old claim and creating the new one, the volume is
#      Released and belongs to nobody. Retain is what stops Kubernetes from
#      reclaiming it - it is set *before* anything is deleted, and verified.
#   2. The platform is down from the scale-down until the new namespace is
#      deployed. There is no way around that: two namespaces cannot mount the
#      same ReadWriteOnce volume.
#
# A database dump is taken first regardless, because "the volume is fine" is a
# belief until something reads it back.
set -euo pipefail

SOURCE="${1:-}"
TARGET="${2:-}"
BACKUP_DIR="${BACKUP_DIR:-./namespace-migration-$(date +%Y%m%d-%H%M%S)}"

if [ -z "$SOURCE" ] || [ -z "$TARGET" ]; then
  echo "usage: $0 <source-namespace> <target-namespace>" >&2
  exit 2
fi
if [ "$SOURCE" = "$TARGET" ]; then
  echo "source and target are the same namespace" >&2
  exit 2
fi

say() { printf '  %s\n' "$*"; }

kubectl get namespace "$SOURCE" >/dev/null

# A target that already runs something is not a target: this would bind volumes
# under a live deployment's feet.
if kubectl get namespace "$TARGET" >/dev/null 2>&1; then
  running="$(kubectl -n "$TARGET" get deployment --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [ "$running" != "0" ]; then
    echo "namespace $TARGET already runs $running deployment(s); refusing" >&2
    exit 1
  fi
fi

mkdir -p "$BACKUP_DIR"
echo
echo "Migrating $SOURCE -> $TARGET"
echo "Backups in $BACKUP_DIR"
echo

echo "1. Backup"
kubectl -n "$SOURCE" get secret,configmap,pvc,deployment,service,networkpolicy -o yaml >"$BACKUP_DIR/source-objects.yaml"
say "objects: $(wc -l <"$BACKUP_DIR/source-objects.yaml") lines"
# A resumed migration finds the platform already stopped, so the dump cannot be
# taken a second time - and does not need to be. SKIP_DUMP=1 says the operator
# has one already; it names the file so the claim is checkable rather than
# taken on trust.
if [ "${SKIP_DUMP:-0}" = "1" ]; then
  say "database dump skipped: ${EXISTING_DUMP:-taken by an earlier run}"
elif kubectl -n "$SOURCE" get deployment postgres >/dev/null 2>&1; then
  kubectl -n "$SOURCE" exec deployment/postgres -- pg_dump -U noryx -d noryx >"$BACKUP_DIR/noryx.sql"
  size="$(wc -c <"$BACKUP_DIR/noryx.sql")"
  # An empty or truncated dump means the safety net does not exist, and the
  # right time to find that out is before anything is deleted.
  if [ "$size" -lt 10000 ]; then
    echo "the database dump is only ${size} bytes; refusing to continue" >&2
    exit 1
  fi
  say "database dump: ${size} bytes"
fi

echo
echo "2. Volumes to Retain, before anything is deleted"
claims="$(kubectl -n "$SOURCE" get pvc -o jsonpath='{range .items[*]}{.metadata.name} {.spec.volumeName}{"\n"}{end}')"
if [ -z "$claims" ]; then
  say "no volumes"
fi
printf '%s\n' "$claims" | while read -r claim volume; do
  [ -z "${claim:-}" ] && continue
  kubectl patch pv "$volume" -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}' >/dev/null
  policy="$(kubectl get pv "$volume" -o jsonpath='{.spec.persistentVolumeReclaimPolicy}')"
  if [ "$policy" != "Retain" ]; then
    echo "$volume is still $policy; refusing to release it" >&2
    exit 1
  fi
  say "$claim -> $volume (Retain)"
done

echo
echo "3. Stopping the platform"
# The routes go first: a namespace that no longer serves must not keep claiming
# the public host, or Traefik has two routers for it and picks one.
kubectl -n "$SOURCE" delete ingressroutes.traefik.io --all --ignore-not-found >/dev/null 2>&1 || true
kubectl -n "$SOURCE" scale deployment --all --replicas=0 >/dev/null
for _ in $(seq 1 60); do
  remaining="$(kubectl -n "$SOURCE" get pods --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  [ "$remaining" = "0" ] && break
  sleep 2
done
say "pods left in $SOURCE: $(kubectl -n "$SOURCE" get pods --no-headers 2>/dev/null | wc -l | tr -d ' ')"

echo
echo "4. Target namespace and its secrets"
kubectl create namespace "$TARGET" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
# Secrets and config maps are the only objects carrying values generated at
# install time - the service token, the database password, the registry
# credentials. Everything else is rebuilt from the manifests by the deployment.
#
# Server-side apply, not client-side: a client-side apply stores the whole
# object in a last-applied-configuration annotation, and the frontend bundle's
# config map is larger than the 256 KiB an annotation may hold. It failed on
# that one map, and under `set -euo pipefail` took the migration down with it -
# after the platform had been stopped and before the volumes were moved.
kubectl -n "$SOURCE" get secret,configmap -o json |
  python3 -c '
import json, sys
document = json.load(sys.stdin)
kept = []
for item in document.get("items", []):
    metadata = item.get("metadata", {})
    name = metadata.get("name", "")
    # Service account tokens belong to their namespace and are reissued there.
    if item.get("type") == "kubernetes.io/service-account-token":
        continue
    if name.startswith("default-token") or name == "kube-root-ca.crt":
        continue
    for field in ("uid", "resourceVersion", "creationTimestamp", "managedFields", "ownerReferences", "namespace", "selfLink", "generation"):
        metadata.pop(field, None)
    metadata["namespace"] = sys.argv[1]
    kept.append(item)
json.dump({"apiVersion": "v1", "kind": "List", "items": kept}, sys.stdout)
' "$TARGET" | kubectl apply --server-side --force-conflicts -f - | sed 's/^/  /'

echo
echo "5. Rebinding the volumes"
printf '%s\n' "$claims" | while read -r claim volume; do
  [ -z "${claim:-}" ] && continue
  size="$(kubectl -n "$SOURCE" get pvc "$claim" -o jsonpath='{.spec.resources.requests.storage}')"
  class="$(kubectl -n "$SOURCE" get pvc "$claim" -o jsonpath='{.spec.storageClassName}')"
  modes="$(kubectl -n "$SOURCE" get pvc "$claim" -o jsonpath='{.spec.accessModes[0]}')"
  kubectl -n "$SOURCE" delete pvc "$claim" --wait=true >/dev/null
  # A released volume keeps pointing at the claim that is gone; until that
  # reference is cleared no new claim can bind, whatever it asks for.
  kubectl patch pv "$volume" --type=json -p '[{"op":"remove","path":"/spec/claimRef"}]' >/dev/null
  cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${claim}
  namespace: ${TARGET}
spec:
  accessModes: ["${modes}"]
  resources:
    requests:
      storage: ${size}
  storageClassName: ${class}
  volumeName: ${volume}
EOF
  for _ in $(seq 1 30); do
    phase="$(kubectl -n "$TARGET" get pvc "$claim" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    [ "$phase" = "Bound" ] && break
    sleep 2
  done
  phase="$(kubectl -n "$TARGET" get pvc "$claim" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  say "$claim in $TARGET: ${phase:-unknown} ($size, $volume)"
  if [ "$phase" != "Bound" ]; then
    echo "$claim did not bind; the volume is retained and the old claim can be recreated from $BACKUP_DIR" >&2
    exit 1
  fi
done

echo
echo "6. Checking the deployments you are about to recreate"
# A manifest that has drifted from the running deployment is invisible until
# something is recreated from it - which is exactly what happens next. PGDATA is
# the one that bit: the manifest named a subdirectory, the running Postgres used
# the volume root, and the new pod ran initdb into the empty subdirectory and
# came up as a pristine database beside the real one.
if kubectl get deployment postgres -n "$SOURCE" >/dev/null 2>&1 || [ -f "$BACKUP_DIR/source-objects.yaml" ]; then
  live_pgdata="$(kubectl -n "$SOURCE" get deployment postgres -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="PGDATA")].value}' 2>/dev/null || true)"
  if [ -n "$live_pgdata" ]; then
    say "the Postgres that was running used PGDATA=$live_pgdata"
    say "the deployment you recreate MUST set the same value, or it will initdb an empty cluster beside the real one"
  else
    say "the Postgres that was running set no PGDATA: its cluster is at the volume root"
    say "the deployment you recreate must not set one either, for the same reason"
  fi
  # Both installations exist: the DC had no PGDATA, EMSE has one. Neither value
  # can be assumed, which is why it is read from the deployment rather than
  # printed from a manifest.
fi

echo
echo "Done. The data is in $TARGET; nothing runs there yet."
echo "Deploy with NAMESPACE=$TARGET, check the platform, then remove $SOURCE:"
echo "  kubectl delete namespace $SOURCE"
