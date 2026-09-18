#!/usr/bin/env bash
#
# Two things about the manifests that nothing else noticed.
#
# On 2026-09-18 three files sat in deploy/k8s/base and in none of its
# kustomization: the nightly backups, the network policies between the
# platform's own components, and the mail bridge. All three were running in
# production, applied by hand, so everything looked correct - a namespace
# rebuilt from the graph would have come back without any of them. Manifest
# drift is invisible precisely until the moment it is expensive.
#
# The same day, a Role addressed to Longhorn's namespace failed the whole
# Kubernetes job, because that namespace does not exist on a stock Kind
# cluster and an object placed in a missing namespace takes the apply down
# with it. The error named the object; it did not name the namespace, and the
# change that introduced it had nothing else wrong with it.
set -euo pipefail

BASE=${1:-deploy/k8s/base}
OVERLAY=${2:-deploy/k8s/ci}
status=0

echo "== every manifest in ${BASE} is in its kustomization"
listed=$(grep -oE '^  - [a-z0-9.-]+\.yaml' "${BASE}/kustomization.yaml" | sed 's/^  - //' | sort)
present=$(find "${BASE}" -maxdepth 1 -name '*.yaml' ! -name 'kustomization.yaml' -exec basename {} \; | sort)
missing=$(comm -23 <(printf '%s\n' "${present}") <(printf '%s\n' "${listed}"))
if [ -n "${missing}" ]; then
  printf '  FAIL  not in the graph, so nothing deploys them:\n' >&2
  printf '        %s\n' ${missing} >&2
  status=1
else
  echo "  ok    $(printf '%s\n' "${present}" | wc -l | tr -d ' ') file(s)"
fi

echo "== every namespace the overlay uses is one the overlay creates"
# Anything else has to pre-exist on the cluster, which is a dependency the
# smoke test cannot satisfy and a customer might not have either.
render=$(kubectl kustomize "${OVERLAY}")
declared=$(printf '%s' "${render}" | python3 -c '
import sys, yaml
print("\n".join(sorted({d["metadata"]["name"] for d in yaml.safe_load_all(sys.stdin)
                        if d and d.get("kind") == "Namespace"})))')
used=$(printf '%s' "${render}" | python3 -c '
import sys, yaml
seen = set()
for d in yaml.safe_load_all(sys.stdin):
    if not d:
        continue
    ns = (d.get("metadata") or {}).get("namespace")
    if ns:
        seen.add(ns)
    # A RoleBinding reaches into another namespace through its subjects.
    for subject in d.get("subjects") or []:
        if subject.get("namespace"):
            seen.add(subject["namespace"])
print("\n".join(sorted(seen)))')
stray=$(comm -23 <(printf '%s\n' "${used}") <(printf '%s\n' "${declared}"))
if [ -n "${stray}" ]; then
  printf '  FAIL  used but never created, so the apply fails on a bare cluster:\n' >&2
  printf '        %s\n' ${stray} >&2
  status=1
else
  echo "  ok    $(printf '%s\n' "${used}" | wc -l | tr -d ' ') namespace(s), all declared"
fi

exit "${status}"
