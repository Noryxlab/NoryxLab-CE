#!/usr/bin/env bash
set -euo pipefail

# Installs Harbor on a dedicated VM.
# Usage:
#   HARBOR_HOSTNAME=harbor.lan HARBOR_ADMIN_PASSWORD='***' ./install-harbor-vm.sh

HARBOR_VERSION="${HARBOR_VERSION:-2.10.2}"
HARBOR_HOSTNAME="${HARBOR_HOSTNAME:-harbor.lan}"
HARBOR_ADMIN_PASSWORD="${HARBOR_ADMIN_PASSWORD:-}"
HARBOR_INSTALL_DIR="${HARBOR_INSTALL_DIR:-/opt/harbor}"

if [ -z "$HARBOR_ADMIN_PASSWORD" ]; then
  echo "HARBOR_ADMIN_PASSWORD is required." >&2
  exit 1
fi

if [ "${EUID}" -ne 0 ]; then
  exec sudo --preserve-env=HARBOR_VERSION,HARBOR_HOSTNAME,HARBOR_ADMIN_PASSWORD,HARBOR_INSTALL_DIR bash "$0" "$@"
fi

apt-get update -y
apt-get install -y ca-certificates curl openssl docker.io docker-compose-plugin jq rsync python3
systemctl enable --now docker

mkdir -p "$HARBOR_INSTALL_DIR"
cd "$HARBOR_INSTALL_DIR"

TARBALL="harbor-offline-installer-v${HARBOR_VERSION}.tgz"
URL="https://github.com/goharbor/harbor/releases/download/v${HARBOR_VERSION}/${TARBALL}"
[ -f "$TARBALL" ] || curl -fsSL -o "$TARBALL" "$URL"

tar -xzf "$TARBALL"
cd harbor
cp -n harbor.yml.tmpl harbor.yml

mkdir -p certs
if [ ! -f certs/harbor.crt ] || [ ! -f certs/harbor.key ]; then
  openssl req -x509 -nodes -days 3650 -newkey rsa:4096 \
    -subj "/CN=${HARBOR_HOSTNAME}" \
    -addext "subjectAltName=DNS:${HARBOR_HOSTNAME}" \
    -keyout certs/harbor.key \
    -out certs/harbor.crt
fi

sed -i "s/^hostname:.*$/hostname: ${HARBOR_HOSTNAME}/" harbor.yml
sed -i "s/^harbor_admin_password:.*$/harbor_admin_password: ${HARBOR_ADMIN_PASSWORD}/" harbor.yml
sed -i 's/^# *https:/https:/' harbor.yml
sed -i 's/^# *  port: 443/  port: 443/' harbor.yml

if ! grep -q '^  certificate:' harbor.yml; then
  sed -i "/^https:/a\\  certificate: ${HARBOR_INSTALL_DIR}/harbor/certs/harbor.crt" harbor.yml
fi
if ! grep -q '^  private_key:' harbor.yml; then
  sed -i "/^https:/a\\  private_key: ${HARBOR_INSTALL_DIR}/harbor/certs/harbor.key" harbor.yml
fi

./install.sh
echo "Harbor installed: https://${HARBOR_HOSTNAME}"
