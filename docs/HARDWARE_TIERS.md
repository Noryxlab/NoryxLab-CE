# Hardware tiers

Noryx exposes named hardware profiles for user workloads, inspired by Domino
hardware tiers.

Users select a tier by name. The UI displays only resource limits; Kubernetes
requests remain an internal scheduling detail.

## Default catalog

| Tier | CPU limit | Memory limit | Ephemeral storage limit | Default |
|---|---:|---:|---:|---|
| Small | 500m | 1Gi | 4Gi | no |
| Standard | 1 | 2Gi | 8Gi | yes |
| Medium | 2 | 4Gi | 16Gi | no |
| Large | 4 | 8Gi | 32Gi | no |

All tiers currently use hidden requests of:

- CPU: `10m`
- memory: `64Mi`
- ephemeral storage: `64Mi`

These deliberately low requests improve cluster utilization. Limits remain
strictly enforced by Kubernetes.

## Scope

Hardware tiers apply to:

- workspaces;
- jobs;
- applications;
- dashboards.

If no tier is supplied, the platform selects `Standard`.

## API

List the public catalog:

```text
GET /api/v1/hardware-tiers
```

The response never exposes internal requests.

Select a tier when creating a workload:

```json
{
  "projectId": "<project-id>",
  "hardwareTier": "medium"
}
```

The backend resolves the tier and rejects unknown identifiers. Clients cannot
submit arbitrary Kubernetes requests or limits.

## Operational note

Low requests permit overcommit. Administrators must monitor actual CPU and
memory usage and adjust workload concurrency or tier limits before enabling
larger catalogs on small clusters.
