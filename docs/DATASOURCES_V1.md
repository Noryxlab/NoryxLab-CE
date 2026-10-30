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
- `POST /api/v1/datasources` — declare a connector to a database that already
  exists: host, port, database, user, and the name of a secret holding the
  password
- `POST /api/v1/dataservices` — create a database the platform runs: an engine
  from `GET /api/v1/datasource-definitions`, a volume, a hardware tier, and
  credentials the platform generates

Both produce a data source, attachable to a project, told apart by `source`
(`internal` for one the platform runs) and by the pod, service and volume an
internal one carries. The interface currently offers only the first; creating
a database is reachable through the API alone.
- `GET /api/v1/datasources/{datasourceID}`
- `PUT /api/v1/datasources/{datasourceID}`
- `DELETE /api/v1/datasources/{datasourceID}`
- `GET /api/v1/datasources/{datasourceID}/access`
- `PUT|DELETE /api/v1/datasources/{datasourceID}/access/{subjectType}/{subjectID}`
- `PUT /api/v1/datasources/{datasourceID}/ownership`
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
`harbor.example.local/noryx-dataservices`.

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

## Who may do what

**Owner** — a person, a team or an organization — grants, transfers and
deletes. **Read** uses the connector in a workspace and sees who else holds
it. **Write** also changes what it points at, through
`PUT /api/v1/datasources/{datasourceID}`.

That second right was a promise with nothing behind it until 2026-10-30: there
was no route that edited a connector, so Write and Read granted exactly the
same thing under two names, and a rights sweep measuring both found it. The
route exists now, and the consequence it carries is why its audit entry names
the fields that moved and says whether the host was one of them: **a host
changed here redirects every workspace that mounts this connector.**

A database the platform runs keeps its own connection details — host, port,
database, user and password were generated when it created the pod, the
service and the volume, not chosen by anybody — so only its name changes, and
anything else is refused with that sentence.
