# Datasources V1

Datasources are connection objects to existing databases.

Scope V1:

- postgres type
- user-owned datasource objects
- project attach/detach
- connection validation endpoint
- env var injection into workloads (workspaces, jobs, apps)

## API

- `GET /api/v1/datasources`
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

## Workload env vars

For each attached datasource:

- `NORYX_DS_<NAME>_TYPE`
- `NORYX_DS_<NAME>_HOST`
- `NORYX_DS_<NAME>_PORT`
- `NORYX_DS_<NAME>_DATABASE`
- `NORYX_DS_<NAME>_USERNAME`
- `NORYX_DS_<NAME>_PASSWORD`
- `NORYX_DS_<NAME>_SSLMODE`
