#!/usr/bin/env bash
set -euo pipefail

if [ "${EUID}" -ne 0 ]; then
  echo "Run as root (for example: sudo bash scripts/vm/create-noryxops.sh <pubkey-file>)" >&2
  exit 1
fi

if [ $# -ne 1 ]; then
  echo "Usage: $0 <pubkey-file>" >&2
  exit 1
fi

PUBKEY_FILE="$1"
if [ ! -f "$PUBKEY_FILE" ]; then
  echo "Public key file not found: $PUBKEY_FILE" >&2
  exit 1
fi

USER_NAME="noryxops"
HOME_DIR="/home/${USER_NAME}"
SUDOERS_FILE="/etc/sudoers.d/90-${USER_NAME}"

if ! id -u "${USER_NAME}" >/dev/null 2>&1; then
  useradd -m -s /bin/bash "${USER_NAME}"
fi

install -d -m 700 -o "${USER_NAME}" -g "${USER_NAME}" "${HOME_DIR}/.ssh"
install -m 600 -o "${USER_NAME}" -g "${USER_NAME}" "$PUBKEY_FILE" "${HOME_DIR}/.ssh/authorized_keys"

# Keep bootstrap simple: passwordless sudo for automation account.
# Can be restricted later once command surface is stabilized.
cat > "${SUDOERS_FILE}" <<EOF
${USER_NAME} ALL=(ALL) NOPASSWD:ALL
EOF
chmod 440 "${SUDOERS_FILE}"

echo "noryxops created and configured."
echo "Validate with: ssh noryxops@<host> 'sudo -n true && echo ok'"
