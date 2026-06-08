# Organizations

## Responsibility boundary

Keycloak is the source of truth for organizations and memberships. NoryxLab
uses stable Keycloak organization and user IDs; it does not duplicate the
membership model in PostgreSQL.

The organization administration API requires a global administrator. Routine
organization management can be performed from the EE administration UI without
opening the Keycloak administration console.

## Mandatory membership

Set the following backend environment variable to require every authenticated
user to belong to at least one enabled Keycloak organization:

```text
NORYX_ORGANIZATION_REQUIRED=true
```

When enabled:

- bearer-token and web-session requests verify organization membership
- a user without an organization receives HTTP `403`
- the response contains `code: organization_required`
- the frontend displays an access-suspended screen with a logout action
- membership results are cached by the backend for 30 seconds

The CE manifest defaults this setting to `false`. The Premyom EE overlay enables
it. Ensure the bootstrap administrator belongs to an organization before
enabling it.

## Administration API

The API proxies organization operations to Keycloak:

- `GET /api/v1/admin/organizations`
- `POST /api/v1/admin/organizations`
- `DELETE /api/v1/admin/organizations/{organizationID}`
- `GET /api/v1/admin/organizations/{organizationID}/members`
- `PUT /api/v1/admin/organizations/{organizationID}/members/{userID}`
- `DELETE /api/v1/admin/organizations/{organizationID}/members/{userID}`

Create request:

```json
{
  "name": "Imt",
  "alias": "imt"
}
```

An organization cannot be deleted while it still contains members. Removing a
user's final membership immediately blocks future API requests after the
membership cache expires.

Organization create/delete and membership add/remove actions are sent to the
advanced audit sink when that EE feature is enabled.

## Bootstrap

`scripts/keycloak/bootstrap-realm.sh` creates the initial organization and adds
the bootstrap administrator. Defaults:

```text
BOOTSTRAP_ORGANIZATION=Imt
BOOTSTRAP_ORGANIZATION_ALIAS=imt
```

The operation is idempotent.

## Verification

Using the temporary bootstrap header:

```bash
BASE=https://datalab.noryxlab.ai

curl -sk -H 'X-Noryx-User: stef' \
  "$BASE/api/v1/admin/organizations"

curl -sk -i -H 'X-Noryx-User: user-without-org' \
  "$BASE/api/v1/projects"
```

The second request must return:

```json
{
  "code": "organization_required",
  "error": "organization membership required"
}
```

## Troubleshooting

- `403 organization_required`: add the user to an enabled organization.
- `503 organization membership verification is unavailable`: configure the
  Keycloak admin client on the backend.
- `502 failed to verify organization membership`: verify Keycloak reachability
  and admin credentials.
- Membership change not immediately visible: wait up to 30 seconds for the
  backend membership cache.
- Organization deletion returns `409`: remove all members first.

