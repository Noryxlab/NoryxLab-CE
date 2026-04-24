#!/usr/bin/env bash
set -euo pipefail

# Mirrors essential runtime images to Harbor so cluster pulls are fully internal.
#
# Usage:
#   ./scripts/ops/sync-images-to-harbor.sh
#   CATALOG_FILE=deploy/images/essential-images.txt ./scripts/ops/sync-images-to-harbor.sh
#
# Prerequisites:
# - docker CLI installed
# - docker login harbor.lan already done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CATALOG_FILE="${CATALOG_FILE:-${ROOT_DIR}/deploy/images/essential-images.txt}"

if ! command -v docker >/dev/null 2>&1; then
  echo "[ERROR] docker is required" >&2
  exit 1
fi

if [ ! -f "${CATALOG_FILE}" ]; then
  echo "[ERROR] catalog file not found: ${CATALOG_FILE}" >&2
  exit 1
fi

echo "[INFO] using catalog: ${CATALOG_FILE}"

while read -r source target; do
  if [[ -z "${source}" || "${source}" == \#* ]]; then
    continue
  fi
  if [[ -z "${target}" ]]; then
    echo "[ERROR] invalid catalog line (missing target): ${source}" >&2
    exit 1
  fi

  echo "[INFO] pull  ${source}"
  docker pull "${source}"

  echo "[INFO] tag   ${source} -> ${target}"
  docker tag "${source}" "${target}"

  echo "[INFO] push  ${target}"
  docker push "${target}"
done < "${CATALOG_FILE}"

echo "[INFO] image sync completed"
