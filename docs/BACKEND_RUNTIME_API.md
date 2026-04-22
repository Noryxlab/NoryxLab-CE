# Backend Runtime API (CE)

This module adds:

- docker image build submission (Kaniko `Job`)
- pod launch submission (`Pod`)
- project-scoped RBAC checks at API level
- Swagger UI + OpenAPI spec

## Endpoints

- `GET /swagger`
- `GET /swagger/openapi.yaml`
- `POST /api/v1/projects`
- `PUT /api/v1/projects/{projectID}/members/{userID}/role`
- `POST /api/v1/builds`
- `POST /api/v1/pods`

## Auth Header

Mutating routes require:

- `X-Noryx-User: <user-id>`

## RBAC model

- project creator is set to `admin`
- `admin` can assign `viewer|editor|admin`
- `editor` and `admin` can submit builds and launch pods

## Quick test

```bash
BASE="https://datalab.noryxlab.ai"

PROJECT_ID=$(curl -sk -X POST "$BASE/api/v1/projects" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: alice' \
  -d '{"name":"demo"}' | jq -r '.id')

curl -sk -X PUT "$BASE/api/v1/projects/$PROJECT_ID/members/bob/role" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: alice' \
  -d '{"role":"editor"}'

curl -sk -X POST "$BASE/api/v1/builds" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: bob' \
  -d '{
    "projectId":"'"$PROJECT_ID"'",
    "gitRepository":"https://github.com/docker-library/hello-world.git",
    "gitRef":"master",
    "dockerfilePath":"Dockerfile",
    "destinationImage":"harbor.lan/noryx-ce/hello-world:test1"
  }'

curl -sk -X POST "$BASE/api/v1/pods" \
  -H 'Content-Type: application/json' \
  -H 'X-Noryx-User: bob' \
  -d '{
    "projectId":"'"$PROJECT_ID"'",
    "image":"busybox:1.36",
    "command":["sh","-c"],
    "args":["echo noryx && sleep 5"]
  }'
```
