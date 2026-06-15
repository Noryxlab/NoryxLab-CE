# Datasources V1

Datasources are connection objects to existing databases.

Scope:

- external PostgreSQL, MySQL/MariaDB and MongoDB connectors
- user-owned datasource objects
- project attach/detach
- connection validation endpoint
- env var injection into workloads (workspaces, jobs, apps)
- read-only internal data-service definition catalog

## API

- `GET /api/v1/datasources`
- `GET /api/v1/datasource-definitions`
- `POST /api/v1/datasources`
- `DELETE /api/v1/datasources/{datasourceID}`
- `POST /api/v1/datasources/{datasourceID}/validate`
- `GET /api/v1/projects/{projectID}/datasources`
- `PUT /api/v1/projects/{projectID}/datasources/{datasourceID}`
- `DELETE /api/v1/projects/{projectID}/datasources/{datasourceID}`

## Example create payload

```json
{
  "name": "risk-pg",
  "type": "postgres",
  "host": "postgres.internal.lan",
  "port": 5432,
  "database": "risk",
  "username": "analyst",
  "passwordSecret": "risk-pg-password",
  "sslMode": "disable"
}
```

Supported external connector types:

- `postgres`: authenticated SQL validation
- `mysql`: TCP endpoint validation
- `mongodb`: TCP endpoint validation

## Internal data services

Internal data services are separate from datasource connection objects. Their
system definitions expose:

- a platform-maintained Harbor image
- an immutable image digest used by deployments
- an immutable, read-only Dockerfile
- the connector type and default port

The initial catalog contains PostgreSQL, MySQL and MongoDB definitions under
`harbor.lan/noryx-dataservices`. Provisioning persistent instances from these
definitions is a subsequent lifecycle-controller phase. A provisioned instance
will generate an attachable datasource automatically.

## Workload env vars

For each attached datasource:

- `NORYX_DS_<NAME>_TYPE`
- `NORYX_DS_<NAME>_HOST`
- `NORYX_DS_<NAME>_PORT`
- `NORYX_DS_<NAME>_DATABASE`
- `NORYX_DS_<NAME>_USERNAME`
- `NORYX_DS_<NAME>_PASSWORD`
- `NORYX_DS_<NAME>_SSLMODE`
