import * as React from 'react';
import { Download, ShieldCheck } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { DataTable, type Column } from '@/components/common/data-table';
import { SearchInput } from '@/components/common/search-input';
import { SectionHeader } from '@/components/common/page-header';
import { useSoftwareInventory } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime } from '@/lib/format';
import { downloadFile } from '@/lib/api/client';
import type { InventoryItem } from '@/lib/api/types';

/**
 * What the platform ships, and under which licences.
 *
 * A customer's compliance officer asks the question, and the honest answer is
 * generated from the resolved dependency graph rather than remembered. The
 * table says for every component where its licence came from, because a list
 * that mixes what was read from a dependency with what somebody typed invites
 * the reader to trust both equally.
 */

const ORIGIN_TONE: Record<string, 'success' | 'neutral' | 'warning'> = {
  detected: 'success',
  declared: 'neutral',
  unresolved: 'warning',
};

export function SoftwareInventorySection() {
  const t = useT();
  const { locale } = useI18n();
  const inventory = useSoftwareInventory();
  const [search, setSearch] = React.useState('');

  const columns: Column<InventoryItem>[] = [
    {
      id: 'name',
      header: t('inventory.component'),
      sortValue: (item) => item.name,
      searchValue: (item) => `${item.name} ${item.licence} ${item.component} ${item.role ?? ''}`,
      cell: (item) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{item.name}</p>
          {item.role ? <p className="truncate text-xs text-muted-foreground">{item.role}</p> : null}
        </div>
      ),
    },
    {
      id: 'version',
      header: t('inventory.version'),
      sortValue: (item) => item.version || null,
      cell: (item) => <span className="font-mono text-xs">{item.version || '—'}</span>,
    },
    {
      id: 'licence',
      header: t('inventory.licence'),
      sortValue: (item) => item.licence,
      cell: (item) => (
        <Badge tone={item.licence.toLowerCase() === 'unknown' ? 'warning' : 'outline'}>{item.licence}</Badge>
      ),
    },
    {
      id: 'component',
      header: t('inventory.partOf'),
      sortValue: (item) => item.component,
      cell: (item) => <span className="text-xs text-muted-foreground">{item.component}</span>,
    },
    {
      id: 'origin',
      header: t('inventory.source'),
      sortValue: (item) => item.origin,
      cell: (item) => (
        <Badge tone={ORIGIN_TONE[item.origin] ?? 'neutral'}>{t(`inventory.origin_${item.origin}` as never)}</Badge>
      ),
    },
  ];

  const unknown = inventory.data?.counts.unknown ?? 0;

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('inventory.title')}
        description={t('inventory.subtitle')}
        actions={
          <span className="flex items-center gap-2">
            <SearchInput value={search} onValueChange={setSearch} label={t('common.search')} className="w-56" />
            <Button
              variant="secondary"
              onClick={() => void downloadFile('/api/v1/admin/software-inventory.csv', 'noryx-software-inventory.csv')}
            >
              <Download aria-hidden />
              CSV
            </Button>
          </span>
        }
      />

      {inventory.data ? (
        <Card>
          <CardContent className="flex flex-wrap items-center gap-x-6 gap-y-2 py-3 text-sm">
            <span className="flex items-center gap-2">
              <ShieldCheck className="size-4 text-muted-foreground" aria-hidden />
              {t('inventory.count', { count: String(inventory.data.counts.total) })}
            </span>
            {/* Stated rather than hidden: a gap in a compliance document gets
                investigated, while a guess gets believed. */}
            <span className={unknown > 0 ? 'text-warning-foreground' : 'text-muted-foreground'}>
              {unknown > 0 ? t('inventory.unknownCount', { count: String(unknown) }) : t('inventory.allKnown')}
            </span>
            <span className="text-muted-foreground">
              {t('inventory.generated')} {formatDateTime(inventory.data.generatedAt, locale)}
            </span>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <DataTable
          data={inventory.data?.items}
          columns={columns}
          rowKey={(item) => `${item.component}-${item.name}`}
          isLoading={inventory.isLoading}
          isError={inventory.isError}
          error={inventory.error}
          onRetry={() => void inventory.refetch()}
          search={search}
        />
      </Card>

      {inventory.data?.note ? (
        <p className="text-xs leading-relaxed text-muted-foreground">{inventory.data.note}</p>
      ) : null}
    </div>
  );
}
