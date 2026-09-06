import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { DataTable, type Column } from '@/components/common/data-table';
import { SectionHeader } from '@/components/common/page-header';
import { adminApi, projectsApi } from '@/lib/api/endpoints';
import { useT } from '@/lib/i18n';
import { downloadFile } from '@/lib/api/client';
import type { UsageTotal } from '@/lib/api/types';

/**
 * Who consumed what.
 *
 * vCPU-hours rather than a share of the cluster: it is the number that means
 * the same thing across machines of different sizes, and the one a cost
 * conversation is actually held in.
 *
 * The peak is shown beside the total on purpose - an average hides the hour
 * that filled the cluster, which is precisely the hour capacity planning is
 * about.
 */
export function PlatformUsageSection() {
  const t = useT();
  const usage = useQuery({ queryKey: ['admin', 'usage'], queryFn: adminApi.usage });
  const projects = useQuery({ queryKey: ['projects'], queryFn: projectsApi.list });

  const names = React.useMemo(() => {
    const map = new Map<string, string>();
    for (const project of projects.data ?? []) map.set(project.id, project.name);
    return map;
  }, [projects.data]);

  const columns: Column<UsageTotal>[] = [
    {
      id: 'project',
      header: t('usage.project'),
      sortValue: (item) => names.get(item.projectId) ?? item.projectId,
      searchValue: (item) => `${names.get(item.projectId) ?? ''} ${item.projectId}`,
      cell: (item) => (
        <span className="truncate font-medium">{names.get(item.projectId) ?? item.projectId}</span>
      ),
    },
    {
      id: 'vcpuHours',
      header: t('usage.vcpuHours'),
      sortValue: (item) => item.vcpuHours,
      cell: (item) => <span className="tabular-nums">{item.vcpuHours.toFixed(1)}</span>,
    },
    {
      id: 'memoryHours',
      header: t('usage.memoryHours'),
      sortValue: (item) => item.memoryGibHours,
      cell: (item) => <span className="tabular-nums">{item.memoryGibHours.toFixed(1)}</span>,
    },
    {
      id: 'peak',
      header: t('usage.peak'),
      sortValue: (item) => item.peakVcpu,
      cell: (item) => (
        <span className="tabular-nums text-muted-foreground">
          {item.peakVcpu.toFixed(1)} vCPU · {item.peakMemoryGib.toFixed(0)} Gi
        </span>
      ),
    },
    {
      id: 'samples',
      header: t('usage.samples'),
      sortValue: (item) => item.samples,
      // Shown because a total built from three measurements and one built from
      // a month of them should not look the same.
      cell: (item) => <span className="tabular-nums text-muted-foreground">{item.samples}</span>,
    },
  ];

  return (
    <section className="space-y-3">
      <SectionHeader
        title={t('usage.title')}
        description={t('usage.hint')}
        actions={
          <Button variant="secondary" onClick={() => void downloadFile('/api/v1/admin/usage.csv', 'noryx-usage.csv')}>
            <Download className="size-4" aria-hidden />
            {t('common.export')}
          </Button>
        }
      />
      <Card>
        <DataTable
          data={usage.data?.items}
          columns={columns}
          rowKey={(item) => item.projectId}
          isLoading={usage.isLoading}
          isError={usage.isError}
          error={usage.error}
          onRetry={() => void usage.refetch()}
        />
      </Card>
    </section>
  );
}
