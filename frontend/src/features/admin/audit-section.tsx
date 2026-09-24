import * as React from 'react';
import {
  Download,
  ScrollText,
} from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
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
  useAuditEvents,
} from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime } from '@/lib/format';
import type {
  AuditEvent,
} from '@/lib/api/types';

export function AuditSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const events = useAuditEvents();
  const [search, setSearch] = React.useState('');

  const columns: Column<AuditEvent>[] = [
    {
      id: 'occurredAt',
      header: t('admin.occurredAt'),
      sortValue: (event) => event.occurredAt,
      cell: (event) => (
        <span className="whitespace-nowrap text-xs text-muted-foreground">
          {formatDateTime(event.occurredAt, locale)}
        </span>
      ),
    },
    {
      id: 'actor',
      header: t('admin.actor'),
      sortValue: (event) => event.actorUserId,
      searchValue: (event) => `${event.actorUserId} ${event.action} ${event.resourceType}`,
      cell: (event) => <span className="truncate font-medium">{event.actorUserId || '—'}</span>,
    },
    {
      id: 'action',
      header: t('admin.action'),
      sortValue: (event) => event.action,
      cell: (event) => <Badge tone="outline">{event.action}</Badge>,
    },
    {
      id: 'resource',
      header: t('rbac.resource'),
      cell: (event) => (
        <span className="truncate text-xs text-muted-foreground">
          {event.resourceType}
          {event.resourceId ? ` · ${event.resourceId.slice(0, 12)}` : ''}
        </span>
      ),
    },
    {
      id: 'outcome',
      header: t('admin.outcome'),
      sortValue: (event) => event.outcome,
      cell: (event) => (
        <Badge tone={event.outcome?.toLowerCase() === 'success' ? 'success' : 'danger'}>
          {event.outcome || '—'}
        </Badge>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('admin.audit')}
        description={t('admin.auditHint')}
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
                  .downloadAudit()
                  .catch((error: unknown) => toast.error(error, t('admin.exportCsv')))
              }
            >
              <Download aria-hidden />
              {t('admin.exportCsv')}
            </Button>
          </div>
        }
      />
      <Card>
        <DataTable
          data={events.data}
          columns={columns}
          rowKey={(event) => event.id}
          isLoading={events.isLoading}
          isError={events.isError}
          error={events.error}
          onRetry={() => void events.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          defaultSort={{ columnId: 'occurredAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={ScrollText}
              title={t('admin.auditEmpty')}
              description={t('admin.auditEmptyHint')}
            />
          }
        />
      </Card>
    </div>
  );
}
