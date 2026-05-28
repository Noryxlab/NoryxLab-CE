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
