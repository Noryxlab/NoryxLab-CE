import * as React from 'react';
import {
  Activity,
  Boxes,
  Database,
  Download,
  Users,
} from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { SearchInput } from '@/components/common/search-input';
import { Button } from '@/components/ui/button';
import {
  Card,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useToast } from '@/components/ui/toast';
import {
  useDataUsage,
} from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatNumber } from '@/lib/format';
import type {
  DataUsageEdge,
  DataUsageNode,
} from '@/lib/api/types';

export function DataGovernanceSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const usage = useDataUsage();
  const [search, setSearch] = React.useState('');

  // The endpoint returns a graph: edges reference nodes by id, so labels are
  // resolved here rather than repeated on every edge.
  const nodesById = React.useMemo(() => {
    const index = new Map<string, DataUsageNode>();
    for (const node of usage.data?.nodes ?? []) index.set(node.id, node);
    return index;
  }, [usage.data]);

  const label = React.useCallback(
    (id: string) => nodesById.get(id)?.label ?? id,
    [nodesById],
  );

  const columns: Column<DataUsageEdge>[] = [
    {
      id: 'from',
      header: t('rbac.subject'),
      sortValue: (edge) => label(edge.from),
      searchValue: (edge) => `${label(edge.from)} ${label(edge.to)}`,
      cell: (edge) => {
        const node = nodesById.get(edge.from);
        return (
          <div className="min-w-0">
            <p className="truncate font-medium">{node?.label ?? edge.from}</p>
            {node?.subLabel ? (
              <p className="truncate text-xs text-muted-foreground">{node.subLabel}</p>
            ) : null}
          </div>
        );
      },
    },
    {
      id: 'relation',
      header: t('rbac.matrix'),
      sortValue: (edge) => edge.relation,
      cell: (edge) => <Badge tone="outline">{edge.relation}</Badge>,
    },
    {
      id: 'to',
      header: t('rbac.resource'),
      sortValue: (edge) => label(edge.to),
      cell: (edge) => {
        const node = nodesById.get(edge.to);
        return (
          <div className="min-w-0">
            <p className="truncate">{node?.label ?? edge.to}</p>
            {node?.kind ? <p className="text-xs text-muted-foreground">{node.kind}</p> : null}
          </div>
        );
      },
    },
    {
      id: 'project',
      header: t('common.project'),
      cell: (edge) => (
        <span className="truncate text-xs text-muted-foreground">
          {edge.projectId ? label(edge.projectId) : '—'}
        </span>
      ),
    },
  ];

  const summary = usage.data?.summary;

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('admin.dataUsage')}
        description={t('admin.dataUsageHint')}
        actions={
          <div className="flex items-center gap-2">
            <SearchInput
              value={search}
              onValueChange={setSearch}
              label={t('common.search')}
              className="w-56"
            />
            <Button
              variant="secondary"
              onClick={() =>
                void adminApi
                  .downloadDataUsage()
                  .catch((error: unknown) => toast.error(error, t('admin.exportCsv')))
              }
            >
              <Download aria-hidden />
              {t('admin.exportCsv')}
            </Button>
          </div>
        }
      />

      <StatGrid>
        <Stat
          icon={Database}
          label={t('nav.datasets')}
          loading={usage.isLoading}
          value={formatNumber(summary?.datasets, locale)}
          hint={
            summary?.hdsDatasets
              ? `${formatNumber(summary.hdsDatasets, locale)} ${t('datasets.classificationHds')}`
              : undefined
          }
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={usage.isLoading}
          value={formatNumber(summary?.projects, locale)}
        />
        <Stat
          icon={Users}
          label={t('home.users')}
          loading={usage.isLoading}
          value={formatNumber(summary?.users, locale)}
          hint={
            summary?.organizations
              ? `${formatNumber(summary.organizations, locale)} ${t('rbac.organizations')}`
              : undefined
          }
        />
        <Stat
          icon={Activity}
          label={t('home.workloads')}
          loading={usage.isLoading}
          value={formatNumber(summary?.workloads, locale)}
        />
      </StatGrid>

      <Card>
        <DataTable
          data={usage.data?.edges}
          columns={columns}
          rowKey={(edge) => `${edge.from}:${edge.relation}:${edge.to}:${edge.projectId}`}
          isLoading={usage.isLoading}
          isError={usage.isError}
          error={usage.error}
          onRetry={() => void usage.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          emptyState={
            <EmptyState icon={Database} title={t('admin.dataUsage')} description={t('admin.dataUsageHint')} />
          }
        />
      </Card>
    </div>
  );
}
