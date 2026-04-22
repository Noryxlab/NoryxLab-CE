#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://datalab.noryxlab.ai}"
NORYX_USER="${NORYX_USER:-stef}"

if ! command -v jq >/dev/null 2>&1; then
  echo "[ERROR] jq is required" >&2
  exit 1
fi

echo "[INFO] Listing workspaces from ${BASE_URL} as ${NORYX_USER}"
body="$(curl -fsSk "${BASE_URL}/api/v1/workspaces" -H "X-Noryx-User: ${NORYX_USER}")"
ids="$(echo "${body}" | jq -r '.items[]?.id')"

if [[ -z "${ids}" ]]; then
  echo "[INFO] No workspaces to delete"
  exit 0
fi

echo "[INFO] Workspaces to delete:"
echo "${ids}" | sed 's/^/  - /'

for id in ${ids}; do
  code="$(curl -sk -o /tmp/noryx-ws-delete.out -w '%{http_code}' -X DELETE "${BASE_URL}/api/v1/workspaces/${id}" -H "X-Noryx-User: ${NORYX_USER}")"
  if [[ "${code}" == "204" ]]; then
    echo "[OK] deleted ${id}"
    continue
  fi
  echo "[WARN] failed to delete ${id} (HTTP ${code})"
  cat /tmp/noryx-ws-delete.out || true
  echo

done

echo "[INFO] Done"
