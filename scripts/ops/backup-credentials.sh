#!/usr/bin/env bash
#
# The credentials a recovery needs, kept where the cluster is not.
#
#   ./scripts/ops/backup-credentials.sh [namespace]
#
# Everything else the platform backs up is useless without this. The objects at
# the backup target can only be reached with the target's own access key; the
# user secrets inside them can only be read with the master key; the accounts
# can only be decrypted with the identity key. All three lived only in the
# cluster - which is precisely what a backup exists to survive. Restoring after
# losing the cluster was therefore impossible, and nothing said so.
#
# This seals those secrets into one encrypted object beside the backup. What
# has to stay outside then shrinks to two things, and they belong in the same
# place as the Keycloak admin password:
#
#   1. the backup target's access key and secret - without them the object
#      cannot be fetched, and they cannot travel inside it;
#   2. NORYX_RECOVERY_KEY - without it the object cannot be read.
#
# Two items on a card. Everything else can be lost with the cluster.
set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
IMAGE="${IDENTITY_BACKUP_IMAGE:-harbor.lan/noryx-ce/noryx-identity-backup:0.1.0}"
KEY_SECRET="${KEY_SECRET:-noryx-recovery-key}"
STAMP="$(date -u +%Y/%m/%d/%H%M%SZ)"

# The secrets a recovery genuinely needs. Named rather than "everything in the
# namespace": a bundle that quietly grows is one nobody can reason about, and
# some of these are regenerated on install rather than restored.
SECRETS="${RECOVERY_SECRETS:-noryx-secrets noryx-service-secrets noryx-backup-target noryx-identity-backup-key harbor-api-credentials noryx-assistant-secrets noryx-alert-secrets}"

if [ -n "${NORYX_RECOVERY_KEY:-}" ]; then
  ${KUBECTL} -n "${NAMESPACE}" create secret generic "${KEY_SECRET}" \
    --from-literal=key="${NORYX_RECOVERY_KEY}" \
    --dry-run=client -o yaml | ${KUBECTL} apply -f - >/dev/null
fi
if ! ${KUBECTL} -n "${NAMESPACE}" get secret "${KEY_SECRET}" >/dev/null 2>&1; then
  echo "no ${KEY_SECRET} secret and no NORYX_RECOVERY_KEY: this bundle would be a plaintext copy of every credential" >&2
  echo "  create one:  openssl rand -hex 32" >&2
  exit 2
fi

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

kept=0
for name in ${SECRETS}; do
  if ${KUBECTL} -n "${NAMESPACE}" get secret "${name}" -o yaml >"${work}/${name}.yaml" 2>/dev/null; then
    # Strip what belongs to this cluster and would fight a restore into
    # another one.
    python3 - "${work}/${name}.yaml" <<'PY'
import re, sys
path = sys.argv[1]
lines = open(path).read().splitlines()
out, skipping = [], False
for line in lines:
    # The namespace goes too: a secret that names its old namespace cannot be
    # applied into a differently named one, and a recovery is exactly when
    # somebody rebuilds under another name - or rehearses in a throwaway.
    if re.match(r"^  (uid|resourceVersion|creationTimestamp|selfLink|generation|namespace):", line):
        continue
    if re.match(r"^  (managedFields|ownerReferences):", line):
        skipping = True
        continue
    if skipping and re.match(r"^  \S", line):
        skipping = False
    if skipping:
        continue
    out.append(line)
open(path, "w").write("\n".join(out) + "\n")
PY
    kept=$((kept + 1))
  else
    echo "  note: ${name} does not exist here, skipped"
  fi
done
if [ "${kept}" = "0" ]; then
  echo "no secret collected; refusing to ship an empty bundle" >&2
  exit 1
fi

target_value() { ${KUBECTL} -n "${NAMESPACE}" get secret noryx-backup-target -o jsonpath="{.data.$1}" | base64 -d; }
ENDPOINT="$(target_value endpoint)"
BUCKET="$(target_value bucket)"
ACCESS_KEY="$(target_value accessKey)"
SECRET_KEY="$(target_value secretKey)"
ENDPOINT_SCHEME="${ENDPOINT%%://*}"
ENDPOINT_HOST="${ENDPOINT#*://}"
SECRET_KEY_ESCAPED="$(printf '%s' "${SECRET_KEY}" | sed -e 's|/|%2F|g' -e 's|+|%2B|g' -e 's|=|%3D|g')"
OBJECT="recovery/${STAMP}/credentials.tar.enc"

# The archive is built in the cluster, not here: the plaintext never touches an
# operator's laptop, and never leaves a file behind if this script is killed.
configmap="noryx-recovery-bundle-$(date -u +%s)"
${KUBECTL} -n "${NAMESPACE}" create configmap "${configmap}" --from-file="${work}" >/dev/null
job="noryx-recovery-backup-$(date -u +%s)"
cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job}
  namespace: ${NAMESPACE}
  labels: { app.kubernetes.io/name: noryx-recovery-backup }
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 600
  template:
    metadata:
      labels: { app.kubernetes.io/name: noryx-recovery-backup }
    spec:
      restartPolicy: Never
      imagePullSecrets: [{ name: harbor-regcred }]
      volumes:
        - { name: bundle, configMap: { name: ${configmap} } }
      containers:
        - name: ship
          image: ${IMAGE}
          command:
            - /bin/sh
            - -c
            - |
              set -e
              cd /bundle
              tar cf - . | openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass env:RECOVERY_KEY -out /tmp/credentials.tar.enc
              mc --config-dir /tmp/.mc cp /tmp/credentials.tar.enc "target/\$BUCKET/\$OBJECT"
              echo "shipped \$OBJECT (\$(wc -c < /tmp/credentials.tar.enc) bytes encrypted)"
          env:
            - { name: HOME, value: "/tmp" }
            - { name: BUCKET, value: "${BUCKET}" }
            - { name: OBJECT, value: "${OBJECT}" }
            - name: MC_HOST_target
              value: "${ENDPOINT_SCHEME}://${ACCESS_KEY}:${SECRET_KEY_ESCAPED}@${ENDPOINT_HOST}"
            - name: RECOVERY_KEY
              valueFrom: { secretKeyRef: { name: ${KEY_SECRET}, key: key } }
          volumeMounts: [{ name: bundle, mountPath: /bundle, readOnly: true }]
EOF

for _ in $(seq 1 60); do
  succeeded="$(${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.succeeded}' 2>/dev/null || true)"
  failed="$(${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.failed}' 2>/dev/null || true)"
  [ -n "${succeeded:-}" ] && [ "${succeeded}" != "0" ] && break
  [ -n "${failed:-}" ] && [ "${failed}" != "0" ] && break
  sleep 5
done
${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" --tail=3 2>/dev/null | sed 's/^/  /' || true
${KUBECTL} -n "${NAMESPACE}" delete "job/${job}" >/dev/null 2>&1 || true
# The config map held every credential in plaintext inside the cluster. It goes
# as soon as the job that read it is done.
${KUBECTL} -n "${NAMESPACE}" delete configmap "${configmap}" >/dev/null 2>&1 || true

echo "  ${kept} secret(s) sealed into ${BUCKET}/${OBJECT}"
echo "  keep two things outside the cluster: the backup target's access key, and NORYX_RECOVERY_KEY"
