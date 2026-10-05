#!/usr/bin/env bash
# Refuse a rendered installation manifest that still contains repository
# defaults. Applying it would create pods that cannot pull images, authenticate
# users, or run the scheduled maintenance jobs.
set -euo pipefail

manifest=${1:-}
if [ -z "$manifest" ] || [ ! -f "$manifest" ]; then
  echo "usage: $0 <rendered-manifest.yaml>" >&2
  exit 2
fi

patterns=(
  'harbor\.example\.local'
  'datalab\.example\.local'
  'auth\.example\.local'
  'CHANGE_ME'
  'KEYCLOAK_IMAGE_PLACEHOLDER'
  'POSTGRES_IMAGE_PLACEHOLDER'
  'BACKUP_IMAGE_PLACEHOLDER'
  'change-me'
  ':latest([[:space:]]|$)'
)

matches=""
for pattern in "${patterns[@]}"; do
  found=$(grep -nE "$pattern" "$manifest" || true)
  [ -z "$found" ] || matches="${matches}${found}"$'\n'
done

if [ -n "$matches" ]; then
  echo "[ERROR] rendered manifest still contains installation defaults:" >&2
  printf '%s' "$matches" >&2
  echo "[ERROR] replace the values in your private installation overlay before applying it." >&2
  exit 1
fi

echo "[OK] rendered manifest has no known repository defaults"
