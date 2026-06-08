# Keycloak Setup (CE)

## Deployment

Keycloak is deployed in `noryx-ce` namespace and exposed under:

- `https://datalab.noryxlab.ai/auth`

The deployment uses `--http-relative-path=/auth`.

## Bootstrap realm and first admin user

Run from a machine with cluster access:

```bash
NS=noryx-ce \
ADMIN_USER=admin \
ADMIN_PASS='<KEYCLOAK_ADMIN_PASSWORD>' \
BOOTSTRAP_USER=stef \
BOOTSTRAP_PASS='<SET_A_PASSWORD>' \
BOOTSTRAP_EMAIL='stef@noryxlab.ai' \
scripts/keycloak/bootstrap-realm.sh
```

This script ensures:

- realm `noryx`
- realm role `noryx-admin`
- public client `noryx-api`
- user `stef` with role `noryx-admin`

## Version and organizations

The platform baseline uses Keycloak `26.6.2`. The `organization` feature,
mandatory membership, and organization bootstrap are installed by the
Enterprise Edition overlay. Keycloak remains the source of truth for identities
when that extension is enabled.

The frontend image bundles `keycloak-js` because Keycloak 26 no longer serves
the legacy `/auth/js/keycloak.js` UMD adapter.

The common backend supports `NORYX_ORGANIZATION_REQUIRED=true`, but CE leaves it
disabled. The EE installation and recovery procedure is maintained in the EE
repository.

Operational details, API examples, guards, and troubleshooting are documented
in `docs/ORGANIZATIONS.md`.

Major Keycloak upgrades require a full database backup and all old Keycloak
nodes to be stopped before the new version migrates the schema. The schema is
not backward compatible after migration.

## API auth config

`noryx-api` expects:

- issuer: `http://keycloak:8080/auth/realms/noryx`
- audience: `noryx-api`

## Getting a token quickly (password grant for tests)

```bash
curl -sk -X POST 'https://datalab.noryxlab.ai/auth/realms/noryx/protocol/openid-connect/token' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=password' \
  --data-urlencode 'client_id=noryx-api' \
  --data-urlencode 'username=stef' \
  --data-urlencode 'password=<SET_A_PASSWORD>'
```

Use `access_token` as:

```bash
-H "Authorization: Bearer <access_token>"
```
