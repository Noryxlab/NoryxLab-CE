#!/usr/bin/env bash
#
# Post-deployment smoke test.
#
# `kubectl rollout status` says the containers started. It says nothing about
# whether the platform works, and this week it said "successfully rolled out"
# four times over a build in which every Enterprise route was dead: they fell
# through to the application shell and answered 200 with HTML.
#
# So each check below exists because something it covers actually shipped
# broken. None of them is a general health check; they are the specific
# questions whose wrong answers reached production.
#
#   ./scripts/ops/smoke_deployment.sh https://datalab.example.local
#
# Options via environment:
#   EXPECT_VERSION           fail unless the API reports this backend version
#   EXPECT_FRONTEND_VERSION  fail unless /version.json reports this one
#   EXPECT_EDITION           "community" or "enterprise"
#   NAMESPACE                Kubernetes namespace, for the log checks
#   SMOKE_RESOLVE_IP         edge IP to use while preserving the public host/SNI
#   SKIP_CLUSTER             set to 1 to run the HTTP checks only
#   SMOKE_INSECURE           set to 1 to accept an unverified certificate
set -uo pipefail

BASE=${1:-${BASE_URL:-}}
NAMESPACE=${NAMESPACE:-noryx}
EXPECT_VERSION=${EXPECT_VERSION:-}
EXPECT_FRONTEND_VERSION=${EXPECT_FRONTEND_VERSION:-}
EXPECT_EDITION=${EXPECT_EDITION:-}
SKIP_CLUSTER=${SKIP_CLUSTER:-0}
SMOKE_INSECURE=${SMOKE_INSECURE:-0}
SMOKE_RESOLVE_IP=${SMOKE_RESOLVE_IP:-}

# The certificate is verified by default.
#
# Every probe here used to pass -k, so the smoke proved a TLS connection was
# possible and nothing about whether it was trustworthy: an expired chain, or
# one issued for another name, would have gone through green. That is a
# production outage this check exists to see coming.
#
# SMOKE_INSECURE=1 remains for installations with a private CA, and says so in
# the output rather than passing silently.
CURL_TLS=""
if [ "$SMOKE_INSECURE" = "1" ]; then
  CURL_TLS="-k"
fi

if [ -z "$BASE" ]; then
  echo "usage: $0 <base-url>   (or set BASE_URL)" >&2
  exit 2
fi
BASE=${BASE%/}
SMOKE_HOST=${BASE#https://}
SMOKE_HOST=${SMOKE_HOST#http://}
SMOKE_HOST=${SMOKE_HOST%%/*}
SMOKE_HOST=${SMOKE_HOST%%:*}
# A string rather than an array, for the same reason CURL_TLS is one: under
# `set -u`, bash 3.2 - which is what macOS ships - treats an empty array
# expansion as an unbound variable and kills the script. bash 4.4 and later do
# not, so this breaks only on a developer's laptop and never on the Linux host
# that runs it after a deployment.
CURL_RESOLVE=""
if [ -n "$SMOKE_RESOLVE_IP" ]; then
  CURL_RESOLVE="--resolve ${SMOKE_HOST}:443:${SMOKE_RESOLVE_IP}"
fi

failures=0
fail() { printf '  FAIL  %s\n' "$1" >&2; failures=$((failures + 1)); }
pass() { printf '  ok    %s\n' "$1"; }

# status METHOD PATH -> HTTP code
status() {
  curl -s ${CURL_RESOLVE} ${CURL_TLS} -o /dev/null -w '%{http_code}' --max-time 20 -X "$1" "${BASE}$2"
}
# content_type PATH -> content type
content_type() {
  curl -s ${CURL_RESOLVE} ${CURL_TLS} -o /dev/null -w '%{content_type}' --max-time 20 "${BASE}$1"
}
body() {
  curl -s ${CURL_RESOLVE} ${CURL_TLS} --max-time 20 "${BASE}$1"
}

echo "Deployment smoke: ${BASE}"
if [ "$SMOKE_INSECURE" = "1" ]; then
  printf '  note  certificate verification is disabled (SMOKE_INSECURE=1)\n'
fi

# 0. Wait for the edge to settle before judging anything.
#
#    This runs seconds after a rollout, while the load balancer can still be
#    routing to a terminating pod. The first version waited only for the root
#    page and only for that check, so everything after it passed by accident of
#    wall-clock ordering - and it reported a 502 on a platform that was healthy
#    a minute later.
#
#    Two consecutive successes, because a single one can catch the moment
#    between two endpoint updates. Nothing below runs until this settles, so a
#    failure means the deployment, not the timing.
settle=0
consecutive=0
while [ "$consecutive" -lt 2 ] && [ "$settle" -lt 30 ]; do
  if [ "$(status GET /)" = "200" ] && [ "$(status GET /config.js)" = "200" ]; then
    consecutive=$((consecutive + 1))
  else
    consecutive=0
  fi
  [ "$consecutive" -lt 2 ] && { settle=$((settle + 1)); sleep 5; }
done
if [ "$consecutive" -lt 2 ]; then
  fail "the platform never answered steadily within $((settle * 5))s; the checks below describe a moving target"
fi

# 1. The edge answers at all, over TLS. An EE deployment once switched Traefik
#    to plain HTTP while HAProxy was doing passthrough, and every page 404ed.
#
#    Measured once. A failure message that re-probes reports a different
#    moment from the one that failed, which is how a check contradicts itself.
root_code=$(status GET /)
if [ "$root_code" = "200" ]; then
  pass "the application is served$([ "$settle" -gt 0 ] && printf ' (after %ss)' "$((settle * 5))")"
else
  fail "the application is not served (HTTP ${root_code})"
fi

# 2. Identity provider discovery. Authentication breaking is invisible from a
#    rollout: the pods are perfectly healthy and nobody can log in.
if [ "$(status GET /auth/realms/noryx/.well-known/openid-configuration)" = "200" ]; then
  pass "identity provider discovery answers"
else
  fail "identity provider discovery does not answer"
fi

# 3. The version actually deployed. A tag can move with no image behind it, and
#    a rollout that times out leaves the previous version serving happily.
version_body=$(body /api/v1/version)
reported_version=$(printf '%s' "$version_body" | sed -n 's/.*"backendVersion":"\([^"]*\)".*/\1/p')
reported_edition=$(printf '%s' "$version_body" | sed -n 's/.*"edition":"\([^"]*\)".*/\1/p')
if [ -z "$reported_version" ]; then
  fail "the version endpoint did not answer with a version"
else
  if [ -n "$EXPECT_VERSION" ] && [ "$reported_version" != "$EXPECT_VERSION" ]; then
    fail "serving ${reported_version}, expected ${EXPECT_VERSION}"
  else
    pass "serving ${reported_version} (${reported_edition:-unknown edition})"
  fi
fi
if [ -n "$EXPECT_EDITION" ] && [ "$reported_edition" != "$EXPECT_EDITION" ]; then
  fail "edition is ${reported_edition:-unset}, expected ${EXPECT_EDITION}"
fi

# 3b. The interface actually served. The backend version says nothing about it:
#     a frontend tag pointing at an older image serves a stale interface while
#     every backend check passes.
frontend_version=$(printf '%s' "$(body /version.json)" | sed -n 's/.*"version"[: ]*"\([^"]*\)".*/\1/p')
if [ -z "$frontend_version" ]; then
  fail "the interface does not report a version"
elif [ -n "$EXPECT_FRONTEND_VERSION" ] && [ "$frontend_version" != "$EXPECT_FRONTEND_VERSION" ]; then
  fail "the interface is ${frontend_version}, expected ${EXPECT_FRONTEND_VERSION}"
else
  pass "the interface is ${frontend_version}"
fi

# 3c. How long the certificate has left.
#
#     Traefik renews from ACME at roughly thirty days remaining, so this is not
#     asking anyone to renew: it reports when renewal has stopped working. A
#     certificate with three weeks left has already missed one, and that miss
#     is the moment worth seeing - not the outage a fortnight later.
if command -v openssl >/dev/null 2>&1; then
  host=${SMOKE_HOST}
  connect_host=${SMOKE_RESOLVE_IP:-$host}
  # A cluster can resolve its public name to an edge it cannot hairpin through.
  # Do not let that optional expiry report block the whole deployment forever.
  # `timeout` is GNU and absent on macOS, where its absence made the whole
  # pipeline fail and this check degrade to a note - on the one certificate it
  # exists to watch. Used when present, skipped when not, because a bounded
  # wait is worth having and a silent skip is not.
  bounded=""
  command -v timeout >/dev/null 2>&1 && bounded="timeout 15"
  not_after=$(echo | $bounded openssl s_client -servername "$host" -connect "${connect_host}:443" 2>/dev/null |
    openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)
  if [ -z "$not_after" ]; then
    printf '  note  could not read the certificate expiry\n'
  else
    expiry_epoch=$(date -d "$not_after" +%s 2>/dev/null || date -j -f '%b %d %T %Y %Z' "$not_after" +%s 2>/dev/null)
    if [ -n "$expiry_epoch" ]; then
      days_left=$(( (expiry_epoch - $(date +%s)) / 86400 ))
      if [ "$days_left" -le 7 ]; then
        fail "the certificate expires in ${days_left} day(s) and has not renewed"
      elif [ "$days_left" -le 21 ]; then
        fail "the certificate expires in ${days_left} day(s): it has missed its renewal window"
      else
        pass "the certificate is valid for ${days_left} more day(s)"
      fi
    fi
  fi
fi

# 4. An unknown API path must answer as an API. When it returned the home page
#    with 200, a missing route was indistinguishable from a working one, which
#    is what hid the Enterprise routes for a whole release.
unknown_type=$(content_type /api/v1/this-endpoint-does-not-exist)
case "$unknown_type" in
  application/json*) pass "an unknown API path answers as an API" ;;
  *) fail "an unknown API path answers ${unknown_type:-nothing}, not JSON" ;;
esac

# 5. Naming yourself in a header is not an identity. This was open, and any
#    caller able to reach the backend could act as any user.
bypass=$(curl -s ${CURL_RESOLVE} ${CURL_TLS} -o /dev/null -w '%{http_code}' --max-time 20 \
  -H 'X-Noryx-User: smoke-test-probe' "${BASE}/api/v1/projects")
if [ "$bypass" = "401" ]; then
  pass "the user header alone does not authenticate"
else
  fail "the user header answered HTTP ${bypass}; it must be refused"
fi

# 6. Enterprise routes. 401 means registered and guarded; 404 means the edition
#    does not ship them. Anything else - and especially HTML - means they are
#    wired nowhere and falling through.
if [ "$EXPECT_EDITION" = "enterprise" ]; then
  for path in /api/v1/admin/backups/runs /api/v1/admin/audit /api/v1/egress/profiles; do
    code=$(status GET "$path")
    type=$(content_type "$path")
    case "$type" in
      text/html*) fail "$path answers HTML: the route is not registered" ;;
      *) if [ "$code" = "401" ] || [ "$code" = "403" ]; then
           pass "$path is registered"
         else
           fail "$path answered HTTP ${code}, expected 401"
         fi ;;
    esac
  done
fi

# 7. The frontend bundle and its declared extensions. An extension can be
#    declared, served, and never mounted - which is how the assistant vanished
#    while every artefact looked correct.
bundle=$(body / | sed -n 's/.*\(\/assets\/index-[A-Za-z0-9_-]*\.js\).*/\1/p' | head -n 1)
if [ -z "$bundle" ]; then
  fail "the application page references no bundle"
else
  if [ "$(status GET "$bundle")" = "200" ]; then
    pass "the application bundle is served"
  else
    fail "the application bundle ${bundle} is not served"
  fi
fi

config=$(body /config.js)
if [ -z "$config" ]; then
  fail "config.js is not served"
else
  pass "config.js is served"
  # Every declared extension must actually be fetchable. A 404 here is silent
  # in the browser: the console logs it and the feature simply is not there.
  printf '%s' "$config" | grep -oE '"/extensions/[A-Za-z0-9_.-]+\.js"' | tr -d '"' | sort -u |
  while IFS= read -r extension; do
    [ -z "$extension" ] && continue
    if [ "$(status GET "$extension")" = "200" ]; then
      pass "extension ${extension} is served"
    else
      fail "extension ${extension} is declared and not served"
    fi
  done
fi

# 8. Background workers. They are started at boot and never mentioned again, so
#    one silently not starting is invisible until the thing it watches fails -
#    which is how three nights passed with no backup.
if [ "$SKIP_CLUSTER" != "1" ] && command -v kubectl >/dev/null 2>&1; then
  logs=$(kubectl -n "$NAMESPACE" logs deployment/noryx-backend --tail=200 2>/dev/null)
  for worker in "workspace reaper started" "job watcher started" "health watcher started"; do
    if printf '%s' "$logs" | grep -q "$worker"; then
      pass "${worker%% started*} is running"
    else
      fail "${worker%% started*} did not start"
    fi
  done
  if printf '%s' "$logs" | grep -q "no alert webhook configured"; then
    printf '  note  no alert webhook is configured: conditions are recorded, nobody is told\n'
  fi
fi

echo
if [ "$failures" -gt 0 ]; then
  echo "${failures} check(s) failed"
  exit 1
fi
echo "Deployment looks healthy"
