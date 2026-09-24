import { useMutation } from '@tanstack/react-query';
import {
  HardDrive,
  ShieldCheck,
  Trash2,
} from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
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
  useStorageEndpoints,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { StorageCapacityPanel } from '@/features/admin/storage-capacity';
import type {
  StorageEndpoint,
} from '@/lib/api/types';

export function StorageSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const endpoints = useStorageEndpoints();

  const test = useMutation({
    mutationFn: (endpointId: string) => adminApi.testStorageEndpoint(endpointId),
    onSuccess: (result) => {
      invalidate(qk.adminStorageEndpoints);
      if (result.reachable) toast.success(t('datasources.testOk'), t('admin.testEndpoint'));
      else toast.error(result.error ?? t('datasources.testFailed'), t('admin.testEndpoint'));
    },
    onError: (error) => toast.error(error, t('admin.testEndpoint')),
  });

  const remove = useMutation({
    mutationFn: (endpointId: string) => adminApi.removeStorageEndpoint(endpointId),
    onSuccess: () => invalidate(qk.adminStorageEndpoints),
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const columns: Column<StorageEndpoint>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (endpoint) => endpoint.name,
      cell: (endpoint) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{endpoint.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">{endpoint.endpoint}</p>
        </div>
      ),
    },
    {
      id: 'purpose',
      header: t('common.type'),
      cell: (endpoint) => (
        <div className="flex flex-wrap gap-1.5">
          <Badge tone="outline">{endpoint.provider}</Badge>
          {endpoint.defaultDataset ? <Badge tone="brand">{t('nav.datasets')}</Badge> : null}
          {endpoint.defaultBackup ? <Badge tone="brand">{t('nav.backups')}</Badge> : null}
        </div>
      ),
    },
    {
      id: 'classification',
      header: t('datasets.classification'),
      cell: (endpoint) =>
        endpoint.classification === 'hds' ? (
          <Badge tone="warning">{t('datasets.classificationHds')}</Badge>
        ) : (
          <Badge tone="outline">{t('datasets.classificationStandard')}</Badge>
        ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (endpoint) => <StatusBadge status={endpoint.status} locale={locale} />,
    },
    {
      id: 'checked',
      header: t('repositories.lastValidated'),
      sortValue: (endpoint) => endpoint.lastCheckedAt ?? null,
      cell: (endpoint) => (
        <span className="text-xs text-muted-foreground">
          {endpoint.lastCheckedAt ? formatRelative(endpoint.lastCheckedAt, locale) : '—'}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      {/* La capacite avant les points de montage : un administrateur vient ici
          parce que quelque chose ne demarre pas, et la reponse est presque
          toujours la place restante. */}
      <StorageCapacityPanel />
      <SectionHeader title={t('admin.storageEndpoints')} description={t('admin.storageEndpointsHint')} />
      <Card>
        <DataTable
          data={endpoints.data}
          columns={columns}
          rowKey={(endpoint) => endpoint.id}
          isLoading={endpoints.isLoading}
          isError={endpoints.isError}
          error={endpoints.error}
          onRetry={() => void endpoints.refetch()}
          emptyState={
            <EmptyState
              icon={HardDrive}
              title={t('admin.storageEndpoints')}
              description={t('admin.storageEndpointsHint')}
            />
          }
          rowActions={(endpoint) => (
            <>
              <DropdownMenuItem onSelect={() => test.mutate(endpoint.id)}>
                <ShieldCheck aria-hidden />
                {t('admin.testEndpoint')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('common.delete'),
                    description: t('admin.storageEndpointsHint'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    confirmationValue: endpoint.name,
                    onConfirm: () => remove.mutateAsync(endpoint.id),
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
      {dialog}
    </div>
  );
}
