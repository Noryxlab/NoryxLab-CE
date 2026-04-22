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
