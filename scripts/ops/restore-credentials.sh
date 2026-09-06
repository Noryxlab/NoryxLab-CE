#!/usr/bin/env bash
#
# Opens the recovery bundle: the credentials a rebuilt platform needs before it
# can restore anything else.
#
#   NORYX_RECOVERY_KEY=... BACKUP_ACCESS_KEY=... BACKUP_SECRET_KEY=... \
#     ./scripts/ops/restore-credentials.sh <object-key> [namespace]
#
# This is the first step of a recovery and the only one that cannot read
# anything from the cluster - there is no cluster yet. Everything it needs comes
# from the card: the backup target's own credentials, and the recovery key.
#
# It applies the secrets into the namespace and stops. What comes next is the
# platform deployment, then the data restore, then the identity restore - each
# of which can now read what it needs from the cluster.
set -euo pipefail

# --apply takes secrets that were extracted elsewhere. A recovery is often two
# machines: one that can reach the backup, one that can reach the cluster.
if [ "${1:-}" = "--apply" ]; then
  directory="${2:-}"
  NAMESPACE="${3:-${NAMESPACE:-noryx}}"
  KUBECTL="${KUBECTL:-kubectl}"
  [ -d "${directory}" ] || { echo "usage: $0 --apply <directory> [namespace]" >&2; exit 2; }
  ${KUBECTL} create namespace "${NAMESPACE}" --dry-run=client -o yaml | ${KUBECTL} apply -f - >/dev/null
  applied=0
  for file in "${directory}"/*.yaml; do
    [ -e "${file}" ] || continue
    ${KUBECTL} -n "${NAMESPACE}" apply -f "${file}" >/dev/null
    applied=$((applied + 1))
    echo "  restored $(basename "${file}" .yaml)"
  done
  [ "${applied}" != "0" ] || { echo "no secret found in ${directory}" >&2; exit 1; }
  echo
  echo "  ${applied} secret(s) restored into ${NAMESPACE}"
  exit 0
fi

OBJECT="${1:-}"
NAMESPACE="${2:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
IMAGE="${IDENTITY_BACKUP_IMAGE:-harbor.lan/noryx-ce/noryx-identity-backup:0.1.0}"
ENDPOINT="${BACKUP_ENDPOINT:-https://cellar-c2.services.clever-cloud.com}"
BUCKET="${BACKUP_BUCKET:-noryx-backup}"

if [ -z "${OBJECT}" ] || [ -z "${NORYX_RECOVERY_KEY:-}" ] || [ -z "${BACKUP_ACCESS_KEY:-}" ] || [ -z "${BACKUP_SECRET_KEY:-}" ]; then
  echo "usage: NORYX_RECOVERY_KEY=... BACKUP_ACCESS_KEY=... BACKUP_SECRET_KEY=... $0 <object-key> [namespace]" >&2
  exit 2
fi

ENDPOINT_SCHEME="${ENDPOINT%%://*}"
ENDPOINT_HOST="${ENDPOINT#*://}"
SECRET_KEY_ESCAPED="$(printf '%s' "${BACKUP_SECRET_KEY}" | sed -e 's|/|%2F|g' -e 's|+|%2B|g' -e 's|=|%3D|g')"

# Fetched and decrypted here rather than in a job: there is no image pull
# secret in a namespace that has just been created, and this is the step that
# creates it.
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

if command -v mc >/dev/null 2>&1 && command -v openssl >/dev/null 2>&1; then
  MC_HOST_target="${ENDPOINT_SCHEME}://${BACKUP_ACCESS_KEY}:${SECRET_KEY_ESCAPED}@${ENDPOINT_HOST}" \
    mc --config-dir "${work}/.mc" cp "target/${BUCKET}/${OBJECT}" "${work}/credentials.tar.enc" >/dev/null
elif command -v docker >/dev/null 2>&1; then
  # As the invoking user: the image runs as an unprivileged account, and a
  # container writing into a directory it does not own fails with a permission
  # error that reads like a credential problem.
  docker run --rm --user "$(id -u):$(id -g)" \
    -e HOME=/tmp -e MC_HOST_target="${ENDPOINT_SCHEME}://${BACKUP_ACCESS_KEY}:${SECRET_KEY_ESCAPED}@${ENDPOINT_HOST}" \
    -v "${work}:/out" --entrypoint sh "${IMAGE}" -c "mc --config-dir /tmp/.mc cp target/${BUCKET}/${OBJECT} /out/credentials.tar.enc" >/dev/null
else
  # Neither on this machine. This is a real constraint rather than an
  # oversight: the step runs before there is a platform to run it in, so it
  # needs a machine with docker or with mc and openssl. On this installation
  # the build VM has docker and the cluster master does not - so extract there,
  # copy the files over, and apply them with --apply.
  echo "this step needs mc and openssl, or docker: run it where one of those exists" >&2
  echo "  then copy the extracted secrets and apply them:  $0 --apply <directory> [namespace]" >&2
  exit 1
fi

EXTRACT_ONLY="${EXTRACT_ONLY:-0}"

decrypt() {
  if command -v openssl >/dev/null 2>&1; then
    NORYX_RECOVERY_KEY="${NORYX_RECOVERY_KEY}" openssl enc -d "$@"
  else
    docker run --rm -i --user "$(id -u):$(id -g)" -e NORYX_RECOVERY_KEY="${NORYX_RECOVERY_KEY}" --entrypoint openssl "${IMAGE}" enc -d "$@"
  fi
}

NORYX_RECOVERY_KEY="${NORYX_RECOVERY_KEY}" \
  openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -pass env:NORYX_RECOVERY_KEY \
  -in "${work}/credentials.tar.enc" | tar xf - -C "${work}"

found=0
for file in "${work}"/*.yaml; do
  [ -e "${file}" ] || continue
  found=$((found + 1))
done
if [ "${found}" = "0" ]; then
  echo "the bundle contained no secret: wrong object, or wrong key" >&2
  exit 1
fi

# The machine that can reach the backup is often not the machine that can reach
# the cluster - on this installation the build VM has docker and no kubectl,
# the master the reverse. So extraction and application are separable, and the
# extracted files are left where the operator can carry them.
destination="${EXTRACT_TO:-${PWD}/recovered-secrets}"
mkdir -p "${destination}"
cp "${work}"/*.yaml "${destination}/"
echo "  ${found} secret(s) extracted to ${destination}"

if [ "${EXTRACT_ONLY:-0}" = "1" ] || ! command -v "${KUBECTL}" >/dev/null 2>&1; then
  echo "  apply them from a machine with kubectl:"
  echo "    $0 --apply ${destination} ${NAMESPACE}"
  exit 0
fi
"$0" --apply "${destination}" "${NAMESPACE}"
