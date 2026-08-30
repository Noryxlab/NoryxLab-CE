import * as React from 'react';
import { useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { CalendarClock, Play, Plus, Trash2 } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { LogViewer } from '@/components/common/log-viewer';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge, StatusBadge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import {
  useCronJobs,
  useEnvironments,
  useHardwareTiers,
  useJobLogs,
  useJobs,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { cronJobsApi, jobsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime, formatDuration, formatRelative } from '@/lib/format';
import { CRON_PRESETS, describeCron, presentTier } from '@/lib/presenters';
import type { CronJob, Job } from '@/lib/api/types';

const TIME_ZONES = ['Europe/Paris', 'UTC', 'Europe/London', 'America/New_York'];

/** Splits a shell command into argv, honouring simple quoting. The API takes
 *  an argv array; the user types one line. */
function toArgv(command: string): string[] {
  const matches = command.trim().match(/(?:[^\s'"]+|'[^']*'|"[^"]*")+/g);
  return (matches ?? []).map((part) => part.replace(/^['"]|['"]$/g, ''));
}

function CreateJobSheet({
  projectId,
  open,
  onOpenChange,
}: {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const environments = useEnvironments();
  const tiers = useHardwareTiers();

  const [name, setName] = React.useState('');
  const [environmentId, setEnvironmentId] = React.useState('');
  const [tierId, setTierId] = React.useState('');
  const [command, setCommand] = React.useState('');
  const [mode, setMode] = React.useState<'once' | 'schedule'>('once');
  const [schedule, setSchedule] = React.useState<string>(CRON_PRESETS[0].value);
  const [timeZone, setTimeZone] = React.useState('Europe/Paris');
  const [touched, setTouched] = React.useState(false);

  const usable = (environments.data ?? []).filter((environment) => environment.destinationImage);

  React.useEffect(() => {
    if (!environmentId && usable.length > 0) setEnvironmentId(usable[0]?.id ?? '');
  }, [usable, environmentId]);

  React.useEffect(() => {
    if (tierId || !tiers.data?.length) return;
    setTierId((tiers.data.find((tier) => tier.default) ?? tiers.data[0])?.id ?? '');
  }, [tiers.data, tierId]);

  React.useEffect(() => {
    if (!open) {
      setName('');
      setCommand('');
      setMode('once');
      setTouched(false);
    }
  }, [open]);

  const commandError = touched && !command.trim() ? t('common.required') : undefined;

  const mutation = useMutation<Job | CronJob>({
    mutationFn: () => {
      const environment = usable.find((candidate) => candidate.id === environmentId);
      const payload = {
        projectId,
        name: name.trim() || undefined,
        // Required by the API; there is no environment id on the wire.
        image: environment?.destinationImage ?? '',
        command: toArgv(command),
        hardwareTier: tierId || undefined,
      };
      return mode === 'schedule'
        ? cronJobsApi.create({ ...payload, schedule, timeZone })
        : jobsApi.create(payload);
    },
    onSuccess: () => {
      invalidate(qk.jobs(projectId), qk.cronJobs(projectId), qk.projects);
      onOpenChange(false);
      toast.success(mode === 'schedule' ? t('jobs.scheduled') : t('jobs.launched'), t('jobs.title'));
    },
    onError: (error) => toast.error(error, t('jobs.createTitle')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setTouched(true);
            if (command.trim() && environmentId) mutation.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>{t('jobs.createTitle')}</SheetTitle>
            <SheetDescription>{t('jobs.createHint')}</SheetDescription>
          </SheetHeader>

          <SheetBody>
            <Field label={t('jobs.nameLabel')}>
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t('jobs.namePlaceholder')}
                maxLength={80}
              />
            </Field>

            <Field label={t('workspaces.environmentLabel')} required>
              <Select
                value={environmentId}
                onValueChange={setEnvironmentId}
                options={usable.map((item) => ({ value: item.id, label: item.name }))}
              />
            </Field>

            <Field label={t('workspaces.tierLabel')} required>
              <Select
                value={tierId}
                onValueChange={setTierId}
                options={(tiers.data ?? []).map((tier) => {
                  const presented = presentTier(tier, locale);
                  return { value: tier.id, label: presented.name, hint: presented.specs };
                })}
              />
            </Field>

            <Field
              label={t('jobs.commandLabel')}
              description={t('jobs.commandHint')}
              error={commandError}
              required
            >
              <Input
                value={command}
                onChange={(event) => setCommand(event.target.value)}
                onBlur={() => setTouched(true)}
                placeholder={t('jobs.commandPlaceholder')}
                className="font-mono text-xs"
              />
            </Field>

            <Field label={t('jobs.modeLabel')}>
              <Select
                value={mode}
                onValueChange={(value) => setMode(value === 'schedule' ? 'schedule' : 'once')}
                options={[
                  { value: 'once', label: t('jobs.modeOnce') },
                  { value: 'schedule', label: t('jobs.modeSchedule') },
                ]}
              />
            </Field>

            {mode === 'schedule' ? (
              <>
                <Field
                  label={t('jobs.scheduleLabel')}
                  description={`${t('jobs.scheduleHint')} — ${describeCron(schedule, locale)}`}
                >
                  <Select
                    value={schedule}
                    onValueChange={setSchedule}
                    options={CRON_PRESETS.map((preset) => ({
                      value: preset.value,
                      label: preset[locale],
                      hint: preset.value,
                    }))}
                  />
                </Field>
                <Field label={t('jobs.timeZoneLabel')}>
                  <Select
                    value={timeZone}
                    onValueChange={setTimeZone}
                    options={TIME_ZONES.map((zone) => ({ value: zone, label: zone }))}
                  />
                </Field>
              </>
            ) : null}
          </SheetBody>

          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              variant="primary"
              loading={mutation.isPending}
              disabled={!command.trim() || !environmentId}
            >
              {mode === 'schedule' ? t('common.save') : t('jobs.create')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

export function JobsPage() {
  const t = useT();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const [creating, setCreating] = React.useState(false);
  const [selectedJobId, setSelectedJobId] = React.useState<string | null>(null);

  const jobs = useJobs(projectId);
  const crons = useCronJobs(projectId);
  const logs = useJobLogs(selectedJobId ?? undefined);

  const removeJob = useMutation({
    mutationFn: (jobId: string) => jobsApi.remove(jobId),
    onSuccess: () => invalidate(qk.jobs(projectId), qk.projects),
    onError: (error) => toast.error(error, t('jobs.deleteTitle')),
  });

  const removeCron = useMutation({
    mutationFn: (cronJobId: string) => cronJobsApi.remove(cronJobId),
    onSuccess: () => invalidate(qk.cronJobs(projectId)),
    onError: (error) => toast.error(error, t('jobs.deleteTitle')),
  });

  const jobColumns: Column<Job>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (job) => job.name || job.jobName,
      searchValue: (job) => `${job.name} ${job.jobName}`,
      cell: (job) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{job.name || job.jobName}</p>
          {job.command?.length ? (
            <p className="truncate font-mono text-xs text-muted-foreground">{job.command.join(' ')}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (job) => job.status,
      cell: (job) => <StatusBadge status={job.status} locale={locale} />,
    },
    {
      id: 'started',
      header: t('common.startedAt'),
      sortValue: (job) => job.createdAt,
      cell: (job) => (
        <span className="text-xs text-muted-foreground">{formatRelative(job.createdAt, locale)}</span>
      ),
    },
    {
      id: 'duration',
      header: t('common.duration'),
      cell: (job) => (
        <span className="text-xs tabular-nums text-muted-foreground">
          {formatDuration(job.createdAt, job.completedAt ?? new Date(), locale)}
        </span>
      ),
    },
  ];

  const cronColumns: Column<CronJob>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (cron) => cron.name,
      searchValue: (cron) => cron.name,
      cell: (cron) => <span className="font-medium">{cron.name}</span>,
    },
    {
      id: 'schedule',
      header: t('jobs.scheduleLabel'),
      cell: (cron) => (
        <div className="min-w-0">
          <p className="text-sm">{describeCron(cron.schedule, locale)}</p>
          <p className="font-mono text-xs text-muted-foreground">{cron.schedule}</p>
        </div>
      ),
    },
    {
      id: 'timezone',
      header: t('jobs.timeZoneLabel'),
      cell: (cron) => <span className="text-xs text-muted-foreground">{cron.timeZone}</span>,
    },
    {
      // The API exposes no last-run timestamp for a schedule, so the column
      // reports whether it is active instead of rendering a permanent dash.
      id: 'suspended',
      header: t('common.status'),
      sortValue: (cron) => (cron.suspended ? 1 : 0),
      cell: (cron) =>
        cron.suspended ? (
          <Badge tone="neutral">{locale === 'fr' ? 'Suspendue' : 'Suspended'}</Badge>
        ) : (
          <Badge tone="success">{locale === 'fr' ? 'Active' : 'Active'}</Badge>
        ),
    },
    {
      id: 'createdAt',
      header: t('common.createdAt'),
      sortValue: (cron) => cron.createdAt,
      cell: (cron) => (
        <span className="text-xs text-muted-foreground">{formatDateTime(cron.createdAt, locale)}</span>
      ),
    },
  ];

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('jobs.title')}
        description={t('jobs.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('jobs.create')}
          </Button>
        }
      />

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('jobs.history')}</CardTitle>
            <CardDescription>{t('jobs.historyHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <DataTable
          data={jobs.data}
          columns={jobColumns}
          rowKey={(job) => job.id}
          isLoading={jobs.isLoading}
          isError={jobs.isError}
          error={jobs.error}
          onRetry={() => void jobs.refetch()}
          defaultSort={{ columnId: 'started', direction: 'desc' }}
          onRowClick={(job) => setSelectedJobId(job.id)}
          emptyState={
            <EmptyState
              icon={Play}
              title={t('jobs.empty')}
              description={t('jobs.emptyHint')}
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('jobs.create')}
                </Button>
              }
            />
          }
          rowActions={(job) => (
            <>
              <DropdownMenuItem onSelect={() => setSelectedJobId(job.id)}>
                {t('common.viewLogs')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('jobs.deleteTitle'),
                    description: t('jobs.deleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    onConfirm: () => removeJob.mutateAsync(job.id),
                  })
                }
              >
                <Trash2 aria-hidden />
                {t('common.delete')}
              </DropdownMenuItem>
            </>
          )}
        />
      </Card>

      {selectedJobId ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('common.logs')}</CardTitle>
            </CardHeaderText>
            <Button variant="ghost" size="sm" onClick={() => setSelectedJobId(null)}>
              {t('common.close')}
            </Button>
          </CardHeader>
          <CardContent>
            <LogViewer
              content={logs.data}
              isLoading={logs.isLoading}
              downloadName={`job-${selectedJobId}.log`}
            />
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('jobs.schedules')}</CardTitle>
            <CardDescription>{t('jobs.schedulesHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <DataTable
          data={crons.data}
          columns={cronColumns}
          rowKey={(cron) => cron.id}
          isLoading={crons.isLoading}
          isError={crons.isError}
          error={crons.error}
          onRetry={() => void crons.refetch()}
          emptyState={
            <EmptyState
              compact
              icon={CalendarClock}
              title={t('jobs.emptySchedules')}
              description={t('jobs.emptySchedulesHint')}
            />
          }
          rowActions={(cron) => (
            <DropdownMenuItem
              destructive
              onSelect={() =>
                ask({
                  title: t('jobs.deleteTitle'),
                  description: t('jobs.deleteWarning'),
                  confirmLabel: t('common.delete'),
                  destructive: true,
                  onConfirm: () => removeCron.mutateAsync(cron.id),
                })
              }
            >
              <Trash2 aria-hidden />
              {t('common.delete')}
            </DropdownMenuItem>
          )}
        />
      </Card>

      {projectId ? (
        <CreateJobSheet projectId={projectId} open={creating} onOpenChange={setCreating} />
      ) : null}
      {dialog}
    </div>
  );
}
