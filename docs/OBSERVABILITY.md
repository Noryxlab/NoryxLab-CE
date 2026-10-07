# Observability (CE)

Default stack installed by Ansible bootstrap:

- Prometheus + Grafana (`kube-prometheus-stack`)
- Loki (technical logs)
- Promtail (node/pod log shipping)

Namespace: `observability` (configurable).

## Retention policy

- Technical logs in Loki: `31 days` (`744h`)
- Product access/audit logs: persisted in PostgreSQL (`audit_events`) with no purge by default

## Installer integration

Role: `ansible/roles/observability`

Enabled in:

- `ansible/playbooks/bootstrap-demo.yml`

Main variables (in `clients/demo.yaml`):

- `observability_enabled`
- `observability_namespace`
- `observability_loki_retention_hours`
- `observability_storage_class`
- `observability_prometheus_size`
- `observability_grafana_size`
- `observability_loki_size`

## Access examples

```bash
# Grafana
kubectl -n observability port-forward svc/kube-prometheus-stack-grafana 3000:80

# Prometheus
kubectl -n observability port-forward svc/kube-prometheus-stack-prometheus 9090:9090

# Loki gateway
kubectl -n observability port-forward svc/loki-gateway 3100:80
```

Grafana default admin password is set in values for lab bootstrap and should be changed for production.

## Consumption must not depend on somebody looking

The usage sampler writes what each project holds every five minutes, and it reads
the platform's own records to do it. Those records are brought up to date by the
reconciliation against the cluster — which, until 2026-10-07, ran only when
somebody opened the workspaces or the projects screen.

So a workspace whose pod had died went on being counted as held, at its full CPU
and memory, until a human happened to load a page. Measured on the EMSE cluster:
**288 consecutive samples — twenty-four hours at five-minute intervals —**
charging one workspace that no longer existed, to a project that no longer
existed either. On a quiet weekend the error is days long, and it is always in
the same direction: too much.

The sampler now reconciles before it measures. One listing against the cluster
every five minutes is the price of the figure being true.

A related choice, left as it is and worth knowing. `occupiesTheCluster` counts an
**unknown** status as occupying, deliberately: for a quota that is the safe
direction, since an unnamed state must not slip past a limit. The sampler shares
that function, so an unknown status also becomes recorded consumption. The
reconciliation above removes the case that actually occurred; if a new status
ever appears without being declared, it will be billed rather than ignored.
