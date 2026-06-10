# Scheduled jobs

Noryx supports immediate jobs and recurring jobs backed by Kubernetes `CronJob`
resources.

## Create a scheduled job

From a project's **Jobs** page:

1. Select the environment image and hardware tier.
2. Enter the command to run.
3. Select **Job planifie**.
4. Enter a standard five-field cron expression and a timezone.
5. Create the schedule.

Example: `0 8 * * 1-5` runs at 08:00 from Monday to Friday in the selected
timezone.

## Runtime behavior

- Scheduled jobs use the same project `/mnt`, attached datasets, datasources,
  user secrets, environment image and hardware tier as immediate jobs.
- The default timezone is `Europe/Paris`.
- Concurrent executions of the same schedule are forbidden.
- A missed execution can start for up to five minutes after its planned time.
- Kubernetes retains up to three successful executions and one failed
  execution. Finished jobs are deleted after 24 hours.
- Deleting a schedule prevents future executions and removes its dedicated
  user-secret copy and Kubernetes-owned execution history.

## API

- `GET /api/v1/cronjobs?projectId=<project-id>`
- `POST /api/v1/cronjobs`
- `DELETE /api/v1/cronjobs/<cronjob-id>`

Creating and deleting schedules requires the same project permission as
launching a job.
