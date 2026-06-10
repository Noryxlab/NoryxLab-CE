# Job history and results

The Jobs page keeps a persistent execution history per project.

- Immediate jobs and executions created by scheduled jobs are discovered from
  Kubernetes.
- Status, launch time and completion time are synchronized into PostgreSQL.
- Up to 2,000 final log lines are captured as the persistent job result when an
  execution succeeds or fails.
- Once captured, the result remains available after the Kubernetes Job and pod
  are cleaned up.
- While a job is starting or running, the Logs action stays on the Jobs page.
  If the pod or container is not ready yet, the UI displays an informational
  message instead of redirecting to Administration.
- Explicitly deleting a job deletes both its Kubernetes resources and its
  persisted history entry.
