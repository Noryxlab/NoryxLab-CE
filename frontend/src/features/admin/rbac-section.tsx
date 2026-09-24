import * as React from 'react';
import {
  Boxes,
  Download,
  ShieldCheck,
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
  useRbacMatrix,
} from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatNumber } from '@/lib/format';
import { presentRole } from '@/lib/presenters';
import { isEnterprise } from '@/lib/config';
import {
  AccessGraph,
  filterAccessCells,
  noAccessFilters,
  type AccessFilters,
} from '@/features/admin/access-graph';
import { RoleMatrixEditor } from '@/features/admin/role-matrix';
import type {
  RbacCell,
} from '@/lib/api/types';

export function RbacSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const matrix = useRbacMatrix();
  const [search, setSearch] = React.useState('');
  // Les criteres vivent ici, au-dessus du graphe et du tableau, parce qu'ils
  // s'appliquent aux deux. La liste filtree est calculee une fois et partagee.
  const [filters, setFilters] = React.useState<AccessFilters>(noAccessFilters);
  const visibleCells = React.useMemo(
    () => filterAccessCells(matrix.data?.cells ?? [], filters),
    [matrix.data?.cells, filters],
  );

  // Cells are self-describing: subject, resource, role, and whether the grant
  // is direct or inherited from an organisation or an ownership.
  const columns: Column<RbacCell>[] = [
    {
      id: 'subject',
      header: t('rbac.subject'),
      sortValue: (cell) => cell.subjectName || cell.subjectId,
      searchValue: (cell) => `${cell.subjectName} ${cell.subjectId} ${cell.resourceName}`,
      cell: (cell) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{cell.subjectName || cell.subjectId}</p>
          <p className="text-xs text-muted-foreground">{cell.subjectType}</p>
        </div>
      ),
    },
    {
      id: 'resource',
      header: t('rbac.resource'),
      sortValue: (cell) => cell.resourceName || cell.resourceId,
      cell: (cell) => (
        <div className="min-w-0">
          <p className="truncate">{cell.resourceName || cell.resourceId}</p>
          <p className="text-xs text-muted-foreground">{cell.resourceType}</p>
        </div>
      ),
    },
    {
      id: 'role',
      header: t('common.role'),
      sortValue: (cell) => cell.role,
      cell: (cell) => <Badge tone="brand">{presentRole(cell.role, locale)}</Badge>,
    },
    {
      id: 'source',
      header: t('ontologies.source'),
      sortValue: (cell) => cell.source,
      cell: (cell) => (
        <span className="flex items-center gap-1.5">
          <Badge tone="outline">{cell.source}</Badge>
          {cell.inherited ? <Badge tone="neutral">{locale === 'fr' ? 'hérité' : 'inherited'}</Badge> : null}
        </span>
      ),
    },
  ];

  const summary = matrix.data?.summary;

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('rbac.matrix')}
        description={t('rbac.matrixHint')}
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
                  .downloadRbacMatrix()
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
          icon={Users}
          label={t('home.users')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.users, locale)}
        />
        <Stat
          icon={ShieldCheck}
          label={t('rbac.organizations')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.organizations, locale)}
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.projects, locale)}
        />
        <Stat
          label={t('rbac.matrix')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.grants, locale)}
          hint={
            summary?.inherited
              ? `${formatNumber(summary.inherited, locale)} ${locale === 'fr' ? 'hérités' : 'inherited'}`
              : undefined
          }
        />
      </StatGrid>

      {/* The table answers "who has access"; an audit asks "why", which is a
          question about paths. Drawn only once the data is filtered enough to
          be readable - see AccessGraph.

          Both read the same filtered list. The criteria used to live inside
          the graph and the table received every cell, so focusing on one
          dataset moved the drawing and left the rows untouched. */}
      {matrix.data ? (
        <AccessGraph
          report={matrix.data}
          filters={filters}
          onFiltersChange={setFilters}
          edges={visibleCells}
        />
      ) : null}

      {/* L'editeur sous la matrice, dans cet ordre : on regarde d'abord qui a
          acces, et on ecrit ensuite la regle. Reserve a l'edition qui sait
          faire respecter une ligne - ailleurs ce serait un ecran qui promet ce
          que la plateforme ne tiendrait pas. */}
      {isEnterprise() ? <RoleMatrixEditor /> : null}

      <Card>
        <DataTable
          data={matrix.data ? visibleCells : undefined}
          columns={columns}
          rowKey={(cell) =>
            `${cell.subjectType}:${cell.subjectId}:${cell.resourceType}:${cell.resourceId}:${cell.role}`
          }
          isLoading={matrix.isLoading}
          isError={matrix.isError}
          error={matrix.error}
          onRetry={() => void matrix.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          emptyState={<EmptyState title={t('rbac.empty')} description={t('rbac.emptyHint')} />}
        />
      </Card>
    </div>
  );
}
