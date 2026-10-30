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

// Un acteur n'est pas toujours une personne.
//
// Trois formes passent dans cette colonne : un nom de compte, « system » pour
// ce que la plateforme fait d'elle-meme, et un composant sous le prefixe
// « service: » - le prefixe existe pour qu'un composant appele comme un
// compte ne puisse jamais etre lu comme cette personne. Les afficher tels
// quels laissait l'auditeur deviner lequel il regardait.
const COMPONENT_PREFIX = 'service:';

function presentActor(actorUserId: string): { label: string; kind: 'person' | 'component' | 'platform' } {
  const actor = actorUserId.trim();
  if (!actor) return { label: '—', kind: 'person' };
  if (actor.startsWith(COMPONENT_PREFIX)) {
    return { label: actor.slice(COMPONENT_PREFIX.length), kind: 'component' };
  }
  if (actor === 'system') return { label: actor, kind: 'platform' };
  // Le secret partage : un composant, mais aucun ne dit lequel.
  if (actor === 'platform-service') return { label: actor, kind: 'component' };
  return { label: actor, kind: 'person' };
}

// Ce que l'evenement dit en plus, en une ligne.
//
// Les details portent le fait utile - le nom de ce qui a ete supprime, si ses
// fichiers sont partis avec, quel justificatif a agi - et l'ecran n'en
// montrait aucun : il restait un type d'objet et un UUID tronque.
const DETAILS_SHOWN_ELSEWHERE = new Set(['name', 'component', 'declaredByVerified']);

function summariseDetails(details: Record<string, unknown> | undefined): string {
  if (!details) return '';
  return Object.entries(details)
    .filter(([key, value]) => !DETAILS_SHOWN_ELSEWHERE.has(key) && value !== null && value !== '')
    .map(([key, value]) => `${key}=${typeof value === 'object' ? JSON.stringify(value) : String(value)}`)
    .join(' · ');
}

function resourceName(event: AuditEvent): string {
  const named = event.details?.name;
  if (typeof named === 'string' && named.trim() !== '') return named;
  return event.resourceId ? event.resourceId.slice(0, 12) : '';
}

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
      searchValue: (event) =>
        `${event.actorUserId} ${event.action} ${event.resourceType} ${resourceName(event)}`,
      cell: (event) => {
        const actor = presentActor(event.actorUserId);
        return (
          <span className="flex items-center gap-1.5">
            <span className="truncate font-medium">{actor.label}</span>
            {actor.kind === 'component' ? (
              <Badge tone="outline" className="shrink-0">
                {t('admin.auditComponent')}
              </Badge>
            ) : null}
            {actor.kind === 'platform' ? (
              <Badge tone="outline" className="shrink-0">
                {t('admin.auditPlatform')}
              </Badge>
            ) : null}
          </span>
        );
      },
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
        <span className="truncate text-xs text-muted-foreground" title={event.resourceId}>
          {event.resourceType}
          {resourceName(event) ? ` · ${resourceName(event)}` : ''}
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
    {
      id: 'details',
      header: t('admin.auditDetails'),
      searchValue: (event) => summariseDetails(event.details),
      cell: (event) => {
        const summary = summariseDetails(event.details);
        if (!summary) return <span className="text-xs text-muted-foreground">—</span>;
        return (
          <span className="block max-w-[28rem] truncate text-xs text-muted-foreground" title={summary}>
            {summary}
          </span>
        );
      },
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
