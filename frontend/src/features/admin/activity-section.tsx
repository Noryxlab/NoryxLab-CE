import { useMutation } from '@tanstack/react-query';
import {
  Activity,
  Boxes,
  Cpu,
  MemoryStick,
  Square,
} from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import {
  Card,
} from '@/components/ui/card';
import { Badge, StatusBadge } from '@/components/ui/badge';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import {
  useAdminExecutions,
  useAdminOverview,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatCpu, formatNumber, formatRelative } from '@/lib/format';
import type {
  Execution,
} from '@/lib/api/types';

export function ActivitySection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const executions = useAdminExecutions();
  const overview = useAdminOverview();

  const stop = useMutation({
    mutationFn: (execution: Execution) => adminApi.killExecution(execution.kind, execution.id),
    onSuccess: () => invalidate(qk.adminExecutions, qk.adminOverview),
    onError: (error) => toast.error(error, t('activity.stopTitle')),
  });

  const columns: Column<Execution>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (execution) => execution.name,
      searchValue: (execution) => execution.name,
      cell: (execution) => <span className="truncate font-medium">{execution.name}</span>,
    },
    {
      id: 'kind',
      header: t('activity.kind'),
      sortValue: (execution) => execution.kind,
      cell: (execution) => <Badge tone="outline">{execution.kind}</Badge>,
    },
    {
      id: 'project',
      header: t('common.project'),
      sortValue: (execution) => execution.projectName || execution.projectId,
      cell: (execution) => (
        <span className="truncate text-xs text-muted-foreground">
          {execution.projectName || execution.projectId}
        </span>
      ),
    },
    {
      id: 'runtime',
      header: t('activity.pods'),
      cell: (execution) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {execution.runtimeName || '—'}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (execution) => <StatusBadge status={execution.status} locale={locale} />,
    },
    {
      id: 'createdAt',
      header: t('common.startedAt'),
      sortValue: (execution) => execution.createdAt,
      cell: (execution) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(execution.createdAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('activity.title')} description={t('activity.subtitle')} />
      <StatGrid>
        <Stat
          icon={Activity}
          label={t('activity.executions')}
          loading={executions.isLoading}
          value={executions.data?.length ?? 0}
        />
        <Stat
          icon={Boxes}
          label={t('activity.pods')}
          loading={overview.isLoading}
          value={formatNumber(overview.data?.workloadMetrics.pods, locale)}
        />
        <Stat
          icon={Cpu}
          label={t('activity.cpuReserved')}
          loading={overview.isLoading}
          value={formatCpu(overview.data?.workloadMetrics.cpuRequestMillicores, locale)}
        />
        <Stat
          icon={MemoryStick}
          label={t('activity.memoryReserved')}
          loading={overview.isLoading}
          value={formatBytes(overview.data?.workloadMetrics.memoryRequestBytes, locale)}
        />
      </StatGrid>
      <Card>
        <DataTable
          data={executions.data}
          columns={columns}
          rowKey={(execution) => `${execution.kind}:${execution.id}`}
          isLoading={executions.isLoading}
          isError={executions.isError}
          error={executions.error}
          onRetry={() => void executions.refetch()}
          defaultSort={{ columnId: 'createdAt', direction: 'desc' }}
          emptyState={
            <EmptyState icon={Activity} title={t('activity.empty')} description={t('activity.emptyHint')} />
          }
          rowActions={(execution) => (
            <DropdownMenuItem
              destructive
              onSelect={() =>
                ask({
                  title: t('activity.stopTitle'),
                  description: t('activity.stopWarning'),
                  confirmLabel: t('workspaces.stop'),
                  destructive: true,
                  onConfirm: () => stop.mutateAsync(execution),
                })
              }
            >
              <Square aria-hidden />
              {t('workspaces.stop')}
            </DropdownMenuItem>
          )}
        />
      </Card>
      {dialog}
    </div>
  );
}
