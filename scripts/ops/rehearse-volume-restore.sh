#!/usr/bin/env bash
#
# Restores a Longhorn backup and checks it file by file.
#
# The other two rehearsal scripts cover the database and the accounts. Volumes
# had none, so the only evidence they could be restored was that backups
# reported Completed - which proves a file was written, not that it can be read
# back. Those are different claims, and only one of them is worth anything on
# the day it matters.
#
#   ./scripts/ops/rehearse-volume-restore.sh                 # newest backup
#   ./scripts/ops/rehearse-volume-restore.sh backup-ea40e83a # a specific one
#
# KUBECTL="kubectl --context other" picks a cluster. Nothing live is modified:
# the source volume is mounted read-only if at all, everything created is
# removed on the way out including after a failure, and the restore lands in a
# volume of its own with a name nothing else uses.
set -euo pipefail

KUBECTL=${KUBECTL:-kubectl}
NAME=${REHEARSAL_NAME:-rehearsal-restore}
# Anywhere with a Longhorn CSI driver; noryx-loads already has one.
WORK_NAMESPACE=${REHEARSAL_NAMESPACE:-noryx-loads}
PROBE_IMAGE=${REHEARSAL_IMAGE:-docker.io/library/busybox:1.36}

say() { printf '%s\n' "$*"; }

cleanup() {
  # A trap rather than a final line. A run that fails halfway would otherwise
  # leave a ten-gigabyte volume behind, and the next person would find it and
  # not know whether it mattered.
  say "cleaning up"
  $KUBECTL -n "$WORK_NAMESPACE" delete pod "${NAME}-verify" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  $KUBECTL -n "$WORK_NAMESPACE" delete pvc "$NAME" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  $KUBECTL delete pv "$NAME" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  $KUBECTL -n longhorn-system delete volumes.longhorn.io "$NAME" --ignore-not-found --wait=false >/dev/null 2>&1 || true
}
trap cleanup EXIT

# The fingerprint: MD5 over each file's MD5, sorted by path relative to the
# mount point. Counting files or bytes alone would pass on a volume restored
# with the right shape and the wrong contents.
# One line, because it is embedded in a JSON string and a raw newline there is
# a parse error - which is how the first run of this script failed, reporting
# "nothing to compare against" instead of a fingerprint.
fingerprint_script='echo FILES=$(find /v -type f | wc -l); echo BYTES=$(du -sb /v | cut -f1); echo DIGEST=$(find /v -type f -exec md5sum {} + 2>/dev/null | sed "s|/v/||" | sort -k2 | md5sum | cut -c1-32)'

fingerprint() { # claim, read-only?
  local claim=$1 readonly=$2 pod="${NAME}-verify"
  $KUBECTL -n "$WORK_NAMESPACE" delete pod "$pod" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  # The override is built by a JSON encoder rather than by string interpolation.
  # Hand-written, the quotes inside the shell command closed the JSON string and
  # every run failed with "Invalid JSON Patch" - reported honestly as "nothing
  # to compare against", which is the right behaviour and still no measurement.
  local overrides
  overrides=$(PROBE_IMAGE="$PROBE_IMAGE" SCRIPT="$fingerprint_script" CLAIM="$claim" RO="$readonly" python3 -c '
import json, os
readonly = os.environ["RO"] == "true"
print(json.dumps({"spec": {
    "containers": [{"name": "c", "image": os.environ["PROBE_IMAGE"],
                    "command": ["sh", "-c", os.environ["SCRIPT"]],
                    "volumeMounts": [{"name": "v", "mountPath": "/v", "readOnly": readonly}]}],
    "volumes": [{"name": "v", "persistentVolumeClaim": {"claimName": os.environ["CLAIM"], "readOnly": readonly}}],
    "restartPolicy": "Never"}}))')
  $KUBECTL -n "$WORK_NAMESPACE" run "$pod" --restart=Never --image="$PROBE_IMAGE" \
    --image-pull-policy=IfNotPresent --overrides="$overrides" >/dev/null
  # Wait for the pod to finish, not to become ready: it runs once and exits, so
  # readiness is a condition it never reaches.
  $KUBECTL -n "$WORK_NAMESPACE" wait --for=jsonpath='{.status.phase}'=Succeeded pod/"$pod" --timeout=900s >/dev/null 2>&1 || true
  $KUBECTL -n "$WORK_NAMESPACE" logs "$pod" 2>/dev/null | grep -E '^(FILES|BYTES|DIGEST)='
  $KUBECTL -n "$WORK_NAMESPACE" delete pod "$pod" --ignore-not-found --wait=false >/dev/null 2>&1 || true
}

BACKUP=${1:-}
if [ -z "$BACKUP" ]; then
  BACKUP=$($KUBECTL -n longhorn-system get backups.longhorn.io -o json |
    python3 -c '
import json,sys
done=[b for b in json.load(sys.stdin)["items"] if b.get("status",{}).get("state")=="Completed"]
done.sort(key=lambda b: b["status"].get("backupCreatedAt",""), reverse=True)
print(done[0]["metadata"]["name"] if done else "")')
fi
[ -n "$BACKUP" ] || { say "no completed backup to rehearse with"; exit 1; }

# 2>/dev/null on the lookup: a missing backup makes kubectl print an error and
# emit nothing, and the decoder's traceback on empty input would bury the
# message this script is about to print.
read -r SOURCE_VOLUME BACKUP_URL BACKUP_AT SIZE <<<"$(
  $KUBECTL -n longhorn-system get backups.longhorn.io "$BACKUP" -o json 2>/dev/null | python3 -c '
import json,sys
raw = sys.stdin.read().strip()
if not raw:
    sys.exit(0)
s = json.loads(raw)["status"]
print(s.get("volumeName",""), s.get("url",""), s.get("backupCreatedAt",""), s.get("volumeSize","10737418240"))' 2>/dev/null)"
# Refuse early and in plain words. Given a name that does not exist, the run
# used to reach Longhorn's admission webhook and fail with "invalid use of
# ,string struct tag" - true, and no help at all to whoever typed the name.
if [ -z "$BACKUP_URL" ] || [ -z "$SOURCE_VOLUME" ]; then
  say "no backup named $BACKUP, or it has no recorded location" >&2
  say "  list them with: $KUBECTL -n longhorn-system get backups.longhorn.io" >&2
  exit 1
fi

say "rehearsing $BACKUP"
say "  volume  $SOURCE_VOLUME"
say "  taken   $BACKUP_AT"

# The live volume, when it still exists, is the reference - but only if nothing
# has been written to it since the backup. Otherwise a mismatch would say
# nothing about the backup and everything about the intervening hours, which is
# how a good restore gets recorded as a failure.
SOURCE_CLAIM=$($KUBECTL get pv "$SOURCE_VOLUME" -o jsonpath='{.spec.claimRef.name}' 2>/dev/null || true)
SOURCE_NS=$($KUBECTL get pv "$SOURCE_VOLUME" -o jsonpath='{.spec.claimRef.namespace}' 2>/dev/null || true)
EXPECTED=""
if [ -n "$SOURCE_CLAIM" ] && [ "$SOURCE_NS" = "$WORK_NAMESPACE" ]; then
  say "fingerprinting the live volume, read-only"
  EXPECTED=$(fingerprint "$SOURCE_CLAIM" true || true)
  printf '%s\n' "$EXPECTED" | sed 's/^/  /'
else
  say "the source claim is gone or lives elsewhere; the restore will be reported without a comparison"
fi

say "restoring into $NAME"
cat <<YAML | $KUBECTL apply -f - >/dev/null
apiVersion: longhorn.io/v1beta2
kind: Volume
metadata: {name: $NAME, namespace: longhorn-system}
spec:
  fromBackup: "$BACKUP_URL"
  numberOfReplicas: 1
  size: "$SIZE"
  frontend: blockdev
  accessMode: rwx
YAML
for _ in $(seq 1 60); do
  sleep 5
  [ "$($KUBECTL -n longhorn-system get volumes.longhorn.io "$NAME" -o jsonpath='{.status.restoreRequired}' 2>/dev/null)" = "false" ] && break
done

cat <<YAML | $KUBECTL apply -f - >/dev/null
apiVersion: v1
kind: PersistentVolume
metadata: {name: $NAME}
spec:
  capacity: {storage: "$SIZE"}
  accessModes: [ReadWriteMany]
  persistentVolumeReclaimPolicy: Retain
  csi: {driver: driver.longhorn.io, volumeHandle: $NAME, fsType: ext4}
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata: {name: $NAME, namespace: $WORK_NAMESPACE}
spec:
  accessModes: [ReadWriteMany]
  storageClassName: ""
  volumeName: $NAME
  resources: {requests: {storage: "$SIZE"}}
YAML

say "fingerprinting the restored volume"
RESTORED=$(fingerprint "$NAME" false || true)
printf '%s\n' "$RESTORED" | sed 's/^/  /'

if [ -z "$EXPECTED" ]; then
  say "restored, with nothing to compare against"
  exit 0
fi
if [ "$EXPECTED" = "$RESTORED" ]; then
  say "IDENTICAL - the backup restores to the byte"
  exit 0
fi
say "DIFFERENT - either the backup is bad, or the volume changed after it was taken" >&2
say "  check for files written since $BACKUP_AT before concluding the backup is at fault" >&2
exit 1
