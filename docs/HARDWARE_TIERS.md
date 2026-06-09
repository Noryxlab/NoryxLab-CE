# Hardware tiers

Noryx exposes named hardware profiles for user workloads, inspired by Domino
hardware tiers.

Users select a tier by name. The UI displays only resource limits; Kubernetes
requests remain an internal scheduling detail.

## Default catalog

| Tier | CPU limit | Memory limit | Ephemeral storage limit | Default |
|---|---:|---:|---:|---|
| 0.5x2 | 0.5 | 2Gi | 4Gi | no |
| 1x4 | 1 | 4Gi | 8Gi | yes |
| 2x8 | 2 | 8Gi | 16Gi | no |
| 4x16 | 4 | 16Gi | 32Gi | no |

All tiers currently use hidden requests of:

- CPU: `100m`
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

If no tier is supplied, the platform selects `1x4`.

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
  "hardwareTier": "2x8"
}
```

The backend resolves the tier and rejects unknown identifiers. Clients cannot
submit arbitrary Kubernetes requests or limits.

## Operational note

Low requests permit overcommit. Administrators must monitor actual CPU and
memory usage and adjust workload concurrency or tier limits before enabling
larger catalogs on small clusters.
