# Datasources V1

Datasources are connection objects to existing databases.

Scope:

- external PostgreSQL, MySQL/MariaDB and MongoDB connectors
- user-owned datasource objects
- project attach/detach
- connection validation endpoint
- env var injection into workloads (workspaces, jobs, apps)
- read-only internal data-service definition catalog
- persistent internal PostgreSQL, MySQL and MongoDB service provisioning

## API

- `GET /api/v1/datasources`
- `GET /api/v1/datasource-definitions`
- `POST /api/v1/datasources`
- `POST /api/v1/dataservices`
- `DELETE /api/v1/datasources/{datasourceID}`
- `POST /api/v1/datasources/{datasourceID}/validate`
- `GET /api/v1/datasources/{datasourceID}/logs`
- `POST /api/v1/datasources/{datasourceID}/restart`
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
`harbor.lan/noryx-dataservices`.

Creating an internal service provisions:

- one `ReadWriteOnce` PVC
- one credentials Secret managed by the platform
- one database pod and one ClusterIP service in `noryx-loads`
- one attachable internal datasource

The creation request accepts a platform hardware tier through `hardwareTier`.
The tier applies the same hidden low requests and visible CPU/RAM limits used
by workspaces, jobs and apps.

Example:

```json
{
  "name": "project-postgres",
  "definitionId": "postgresql-17",
  "database": "noryx",
  "username": "noryx",
  "storageSize": "10Gi",
  "hardwareTier": "1x4"
}
```

The generated password is encrypted in the Noryx store and injected into
attached workloads through the usual datasource environment variables. It is
not listed, revealed or directly editable in the Secrets UI.

Deleting an internal datasource is destructive: the service, pod, Kubernetes
credentials Secret, Noryx managed Secret and persistent volume claim are
deleted. The UI requires an explicit confirmation that stored data will be
destroyed. Deletion is refused while the datasource remains attached to any
project.

Internal datasource status includes the Kubernetes phase, reason, message,
restart count and start date. Owners can retrieve the last 500 pod log lines
and request a controlled pod restart. The persistent volume and ClusterIP
service survive this restart.

NetworkPolicies allow Noryx user workloads and the Noryx backend to connect to
internal services only on ports `5432`, `3306` and `27017`.

## Workload env vars

For each attached datasource:

- `NORYX_DS_<NAME>_TYPE`
- `NORYX_DS_<NAME>_HOST`
- `NORYX_DS_<NAME>_PORT`
- `NORYX_DS_<NAME>_DATABASE`
- `NORYX_DS_<NAME>_USERNAME`
- `NORYX_DS_<NAME>_PASSWORD`
- `NORYX_DS_<NAME>_SSLMODE`
