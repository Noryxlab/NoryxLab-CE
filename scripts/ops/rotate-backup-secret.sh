#!/usr/bin/env bash
#
# Rotates the object-storage secret that every backup depends on.
#
# One credential serves four consumers - the database and identity backups and
# the Longhorn backups, on both installations - so it cannot be rotated for one
# without being rotated for all, and a leak of any single one compromises every
# backup of both platforms. That is worth fixing separately; this script exists
# because it has to be rotated today.
#
# Cellar issues one key pair per bucket and only the secret can be renewed, so
# this is a hard cutover: the moment the console regenerates, the old secret is
# dead and every consumer is broken until updated. The window is therefore
# measured in seconds and all four are updated from one command.
#
#   ./rotate-backup-secret.sh ~/Downloads/cellar-...-s3cfg.txt
#
# The new secret is read from the file and never printed. The mistake this
# rotation is repairing was a masking filter that did not match the field name.
set -euo pipefail

CONFIG=${1:-}
if [ -z "$CONFIG" ] || [ ! -f "$CONFIG" ]; then
  echo "usage: $0 <fichier s3cfg telecharge depuis Clever>" >&2
  exit 2
fi

ACCESS=$(sed -nE 's/^access_key *= *(.+)$/\1/p' "$CONFIG" | head -1)
SECRET=$(sed -nE 's/^secret_key *= *(.+)$/\1/p' "$CONFIG" | head -1)
if [ -z "$ACCESS" ] || [ -z "$SECRET" ]; then
  echo "le fichier ne contient pas access_key/secret_key" >&2
  exit 1
fi
# L'empreinte, jamais la valeur : de quoi verifier qu'on a bien pris le nouveau
# fichier sans que le secret traverse un terminal ou un journal.
echo "access_key : ${ACCESS:0:6}...  secret : empreinte $(printf %s "$SECRET" | shasum -a 256 | cut -c1-12)"

b64() { printf %s "$1" | base64 | tr -d '\n'; }

# Le correctif applique au patch lui-meme : la charge JSON passe par l'entree
# standard, jamais par les arguments. Envoyee en argument a travers ssh, elle
# est redecoupee par le shell distant et kubectl finit par chercher un secret
# dont le nom est le JSON. Rien n'avait ete modifie, mais l'erreur ressemblait
# a une rotation a moitie faite - ce qui est le pire moment pour douter.
apply_patch() {
  local runner="$1" namespace="$2" name="$3" payload="$4"
  printf %s "$payload" | $runner -n "$namespace" patch secret "$name" \
    --type merge --patch-file /dev/stdin >/dev/null
}

patch_platform() {
  local label="$1" runner="$2"
  echo "== $label"
  apply_patch "$runner" noryx noryx-backup-target \
    "{\"data\":{\"secretKey\":\"$(b64 "$SECRET")\",\"accessKey\":\"$(b64 "$ACCESS")\"}}"
  echo "   noryx-backup-target mis a jour"

  if $runner -n longhorn-system get secret noryx-longhorn-backup >/dev/null 2>&1; then
    apply_patch "$runner" longhorn-system noryx-longhorn-backup \
      "{\"data\":{\"AWS_SECRET_ACCESS_KEY\":\"$(b64 "$SECRET")\",\"AWS_ACCESS_KEY_ID\":\"$(b64 "$ACCESS")\"}}"
    echo "   noryx-longhorn-backup mis a jour"
    # Longhorn relit le secret quand la cible est touchee. Sans cela il garde
    # l'ancienne valeur jusqu'a son prochain cycle, et une sauvegarde lancee
    # entre-temps echoue sans raison lisible.
    $runner -n longhorn-system annotate backuptargets.longhorn.io default \
      noryx.io/secret-rotated-at="$(date -u +%FT%TZ)" --overwrite >/dev/null
    echo "   cible de sauvegarde rafraichie"
  fi
}

# Le DC repond sur le LAN quand le tunnel n'est pas monte ; EMSE par ssh.
DC_KUBECTL=${DC_KUBECTL:-"kubectl --context noryx-test"}
EMSE_SSH=${EMSE_SSH:-noryxops@10.210.53.12}

patch_platform "DC" "$DC_KUBECTL"
patch_platform "EMSE" "ssh $EMSE_SSH sudo kubectl"

echo
echo "Les quatre consommateurs sont a jour."
echo "Verifier avant de considerer la rotation faite :"
echo "  - une sauvegarde Longhorn manuelle sur chaque plateforme"
echo "  - le prochain passage nocturne de la base et de l'identite"
echo "Une cible de sauvegarde qui reste 'available' ne prouve rien : elle peut"
echo "avoir ete validee avant la rotation. Seule une ecriture tranche."
