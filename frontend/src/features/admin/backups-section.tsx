import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  AlertTriangle,
  HardDrive,
  Play,
} from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Badge, StatusBadge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useToast } from '@/components/ui/toast';
import {
  useBackupRuns,
  useBackupStatus,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatDateTime, formatRelative } from '@/lib/format';
import type {
  BackupReport,
  BackupRun,
} from '@/lib/api/types';

export function BackupsSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();

  const status = useBackupStatus();
  const runs = useBackupRuns();

  const [endpoint, setEndpoint] = React.useState('');
  const [bucket, setBucket] = React.useState('');
  const [prefix, setPrefix] = React.useState('');
  const [region, setRegion] = React.useState('');
  const [accessKey, setAccessKey] = React.useState('');
  const [secretKey, setSecretKey] = React.useState('');
  const [encryptionKeyId, setEncryptionKeyId] = React.useState('');

  React.useEffect(() => {
    if (!status.data) return;
    setEndpoint(status.data.endpoint ?? '');
    setBucket(status.data.bucket ?? '');
    setPrefix(status.data.prefix ?? '');
    setRegion(status.data.region ?? '');
  }, [status.data]);

  const save = useMutation({
    mutationFn: () =>
      adminApi.saveBackupConfig({
        endpoint: endpoint.trim(),
        bucket: bucket.trim(),
        prefix: prefix.trim(),
        region: region.trim(),
        accessKey,
        secretKey,
        encryptionKeyId: encryptionKeyId.trim(),
      }),
    onSuccess: () => {
      invalidate(qk.adminBackupStatus);
      setAccessKey('');
      setSecretKey('');
      toast.success(t('common.save'), t('admin.backups'));
    },
    onError: (error) => toast.error(error, t('admin.backupConfigure')),
  });

  const run = useMutation({
    mutationFn: () => adminApi.runBackup(),
    onSuccess: () => {
      invalidate(qk.adminBackupRuns);
      toast.success(t('admin.backupRun'), t('admin.backups'));
    },
    onError: (error) => toast.error(error, t('admin.backupRun')),
  });

  // The report is a JSON string, and its `warnings` array is how a run
  // declares that it succeeded without actually copying anything. Surfacing
  // it is the difference between a backup and the belief that there is one.
  function parseReport(backup: BackupRun): BackupReport | null {
    if (!backup.report) return null;
    try {
      return JSON.parse(backup.report) as BackupReport;
    } catch {
      return null;
    }
  }

  const columns: Column<BackupRun>[] = [
    {
      id: 'startedAt',
      header: t('common.startedAt'),
      sortValue: (backup) => backup.startedAt,
      cell: (backup) => (
        <span className="text-xs text-muted-foreground">{formatDateTime(backup.startedAt, locale)}</span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (backup) => {
        const report = parseReport(backup);
        const warnings = report?.warnings ?? [];
        return (
          <span className="flex flex-wrap items-center gap-1.5">
            <StatusBadge status={backup.status} locale={locale} />
            {warnings.length > 0 ? (
              <Badge tone="warning" title={warnings.join('\n')}>
                <AlertTriangle className="size-3" aria-hidden />
                {warnings.length}
              </Badge>
            ) : null}
          </span>
        );
      },
    },
    {
      id: 'size',
      header: t('common.size'),
      align: 'right',
      sortValue: (backup) => parseReport(backup)?.bytes ?? 0,
      cell: (backup) => (
        <span className="tabular-nums text-muted-foreground">
          {formatBytes(parseReport(backup)?.bytes, locale)}
        </span>
      ),
    },
    {
      id: 'object',
      header: t('admin.bucket'),
      cell: (backup) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {backup.bucket}/{backup.objectKey}
        </span>
      ),
    },
    {
      id: 'error',
      header: t('common.error'),
      cell: (backup) =>
        backup.error ? (
          <span className="line-clamp-2 max-w-xs text-xs text-danger">{backup.error}</span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
  ];

  const configured = status.data?.configured === true;

  // Warnings from the most recent run, hoisted into a banner: a run that
  // reports success while copying nothing is the failure mode that matters.
  const latestWarnings = React.useMemo(() => {
    const latest = [...(runs.data ?? [])].sort(
      (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
    )[0];
    if (!latest) return [];
    return parseReport(latest)?.warnings ?? [];
  }, [runs.data]);

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('admin.backups')}
        description={t('admin.backupsHint')}
        actions={
          <Button
            variant="primary"
            disabled={!configured}
            loading={run.isPending}
            onClick={() => run.mutate()}
          >
            <Play aria-hidden />
            {t('admin.backupRun')}
          </Button>
        }
      />

      <StatGrid className="lg:grid-cols-3">
        <Stat
          label={t('common.status')}
          loading={status.isLoading}
          value={configured ? t('admin.backupConfigured') : t('admin.backupNotConfigured')}
        />
        <Stat label={t('admin.bucket')} loading={status.isLoading} value={status.data?.bucket ?? '—'} />
        <Stat
          label={t('admin.lastUpdated')}
          loading={status.isLoading}
          value={status.data?.updatedAt ? formatRelative(status.data.updatedAt, locale) : '—'}
        />
      </StatGrid>

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('admin.backupConfigure')}</CardTitle>
            <CardDescription>{t('datasets.credentialsHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Field label={t('admin.endpoint')} className="sm:col-span-2 lg:col-span-3" required>
            <Input
              value={endpoint}
              onChange={(event) => setEndpoint(event.target.value)}
              placeholder="https://s3.example.com"
              className="font-mono text-xs"
            />
          </Field>
          <Field label={t('admin.bucket')} required>
            <Input value={bucket} onChange={(event) => setBucket(event.target.value)} />
          </Field>
          <Field label={t('admin.prefix')}>
            <Input value={prefix} onChange={(event) => setPrefix(event.target.value)} />
          </Field>
          <Field label={t('admin.region')}>
            <Input value={region} onChange={(event) => setRegion(event.target.value)} />
          </Field>
          <Field label={t('admin.accessKey')}>
            <Input
              value={accessKey}
              onChange={(event) => setAccessKey(event.target.value)}
              autoComplete="off"
            />
          </Field>
          <Field label={t('admin.secretKey')} description={t('datasets.credentialsHint')}>
            <Input
              type="password"
              value={secretKey}
              onChange={(event) => setSecretKey(event.target.value)}
              autoComplete="new-password"
            />
          </Field>
          <Field label={t('admin.encryptionKey')}>
            <Input
              value={encryptionKeyId}
              onChange={(event) => setEncryptionKeyId(event.target.value)}
            />
          </Field>
        </CardContent>
        <CardFooter className="justify-end">
          <Button
            variant="primary"
            loading={save.isPending}
            disabled={!endpoint.trim() || !bucket.trim()}
            onClick={() => save.mutate()}
          >
            {t('common.save')}
          </Button>
        </CardFooter>
      </Card>

      {latestWarnings.length > 0 ? (
        <div
          role="alert"
          className="flex items-start gap-2.5 rounded-lg border border-warning/40 bg-warning-subtle px-4 py-3"
        >
          <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden />
          <div className="min-w-0 space-y-1">
            <p className="text-sm font-medium text-warning-foreground">
              {t('admin.backupIncomplete')}
            </p>
            <ul className="list-inside list-disc space-y-0.5 text-xs leading-relaxed text-warning-foreground">
              {latestWarnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </div>
        </div>
      ) : null}

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('admin.backupHistory')}</CardTitle>
          </CardHeaderText>
        </CardHeader>
        <DataTable
          data={runs.data}
          columns={columns}
          rowKey={(backup) => backup.id}
          isLoading={runs.isLoading}
          isError={runs.isError}
          error={runs.error}
          onRetry={() => void runs.refetch()}
          defaultSort={{ columnId: 'startedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={HardDrive}
              title={t('admin.backupEmpty')}
              description={t('admin.backupEmptyHint')}
            />
          }
        />
      </Card>
    </div>
  );
}
