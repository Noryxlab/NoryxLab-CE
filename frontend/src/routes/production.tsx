import * as React from 'react';
import { Link } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { ExternalLink, History, Rocket } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { SearchInput } from '@/components/common/search-input';
import { CopyButton } from '@/components/common/copy-button';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge, StatusBadge, describeStatus } from '@/components/ui/badge';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import { useAppRevisions, useProductionApps, useProjects, qk, useInvalidate } from '@/lib/api/queries';
import { appsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime } from '@/lib/format';
import { presentAccessMode } from '@/lib/presenters';
import type { App, AppRevision } from '@/lib/api/types';

export function ProductionPage() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const production = useProductionApps();
  const projects = useProjects();
  const [search, setSearch] = React.useState('');
  const [selected, setSelected] = React.useState<App | null>(null);
  const revisions = useAppRevisions(selected?.id);

  const projectName = React.useCallback(
    (projectId: string) => projects.data?.find((project) => project.id === projectId)?.name ?? projectId,
    [projects.data],
  );

  const rollback = useMutation({
    mutationFn: (input: { appId: string; revisionId: string }) =>
      appsApi.rollback(input.appId, input.revisionId),
    onSuccess: () => {
      invalidate(qk.production, qk.apps());
      toast.success(t('apps.rollback'), t('production.title'));
    },
    onError: (error) => toast.error(error, t('apps.rollback')),
  });

  const running = (production.data ?? []).filter(
    (app) => describeStatus(app.status).tone === 'success',
  ).length;
  const broadlyAccessible = (production.data ?? []).filter((app) =>
    ['public', 'authenticated', 'organization'].includes(String(app.accessMode).toLowerCase()),
  ).length;

  const columns: Column<App>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (app) => app.name || app.slug,
      searchValue: (app) => `${app.name} ${app.slug}`,
      cell: (app) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{app.name || app.slug}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">/{app.slug}</p>
        </div>
      ),
    },
    {
      id: 'project',
      header: t('common.project'),
      sortValue: (app) => projectName(app.projectId),
      searchValue: (app) => projectName(app.projectId),
      cell: (app) => (
        <Link
          to={`/projects/${app.projectId}/apps`}
          className="text-xs text-muted-foreground hover:text-brand hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          {projectName(app.projectId)}
        </Link>
      ),
    },
    {
      id: 'revision',
      header: t('production.revision'),
      align: 'right',
      sortValue: (app) => app.activeRevision ?? 0,
      cell: (app) => (
        <span className="tabular-nums text-muted-foreground">#{app.activeRevision ?? 1}</span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (app) => app.status,
      cell: (app) => <StatusBadge status={app.status} locale={locale} />,
    },
    {
      id: 'access',
      header: t('production.access'),
      cell: (app) => <Badge tone="outline">{presentAccessMode(app.accessMode, locale)}</Badge>,
    },
    {
      id: 'url',
      header: 'URL',
      cell: (app) =>
        app.accessUrl ? (
          <span className="flex items-center gap-1">
            <a
              href={app.accessUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="truncate text-xs text-brand hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              onClick={(event) => event.stopPropagation()}
            >
              {app.accessUrl.replace(/^https?:\/\//, '')}
            </a>
            <CopyButton value={app.accessUrl} />
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      id: 'publishedAt',
      header: t('production.publishedAt'),
      sortValue: (app) => app.publishedAt ?? null,
      cell: (app) => (
        <span className="text-xs text-muted-foreground">
          {app.publishedAt ? formatDateTime(app.publishedAt, locale) : '—'}
        </span>
      ),
    },
  ];

  const revisionColumns: Column<AppRevision>[] = [
    {
      id: 'number',
      header: t('production.revision'),
      sortValue: (revision) => revision.number,
      cell: (revision) => (
        <span className="flex items-center gap-2 font-medium tabular-nums">
          #{revision.number}
          {revision.active ? <Badge tone="success">{t('environments.activeRevision')}</Badge> : null}
        </span>
      ),
    },
    {
      id: 'image',
      header: 'Image',
      cell: (revision) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {revision.snapshot?.image ?? '—'}
        </span>
      ),
    },
    {
      id: 'publishedBy',
      header: t('production.publishedBy'),
      cell: (revision) => (
        <span className="text-xs text-muted-foreground">{revision.publishedBy || '—'}</span>
      ),
    },
    {
      id: 'publishedAt',
      header: t('production.publishedAt'),
      sortValue: (revision) => revision.publishedAt,
      cell: (revision) => (
        <span className="text-xs text-muted-foreground">
          {formatDateTime(revision.publishedAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-5">
      <PageHeader title={t('production.title')} description={t('production.subtitle')} />

      <StatGrid className="lg:grid-cols-3">
        <Stat
          icon={Rocket}
          label={t('production.published')}
          loading={production.isLoading}
          value={production.data?.length ?? 0}
        />
        <Stat label={t('production.running')} loading={production.isLoading} value={running} />
        <Stat
          label={t('production.public')}
          loading={production.isLoading}
          value={broadlyAccessible}
        />
      </StatGrid>

      {production.data && production.data.length > 0 ? (
        <SearchInput
          value={search}
          onValueChange={setSearch}
          label={t('common.search')}
          className="max-w-xs"
        />
      ) : null}

      <Card>
        <DataTable
          data={production.data}
          columns={columns}
          rowKey={(app) => app.id}
          isLoading={production.isLoading}
          isError={production.isError}
          error={production.error}
          onRetry={() => void production.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          onRowClick={(app) => setSelected(app)}
          defaultSort={{ columnId: 'publishedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={Rocket}
              title={t('production.empty')}
              description={t('production.emptyHint')}
              action={
                <Button variant="secondary" asChild>
                  <Link to="/projects">{t('nav.projects')}</Link>
                </Button>
              }
            />
          }
          rowActions={(app) => (
            <>
              <DropdownMenuItem onSelect={() => setSelected(app)}>
                <History aria-hidden />
                {t('production.revisionHistory')}
              </DropdownMenuItem>
              {app.accessUrl ? (
                <DropdownMenuItem asChild>
                  <a href={app.accessUrl} target="_blank" rel="noopener noreferrer">
                    <ExternalLink aria-hidden />
                    {t('common.open')}
                  </a>
                </DropdownMenuItem>
              ) : null}
            </>
          )}
        />
      </Card>

      {selected ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>
                {t('production.revisionHistory')} — {selected.name || selected.slug}
              </CardTitle>
              <CardDescription>{projectName(selected.projectId)}</CardDescription>
            </CardHeaderText>
            <Button variant="ghost" size="sm" onClick={() => setSelected(null)}>
              {t('common.close')}
            </Button>
          </CardHeader>
          <DataTable
            data={revisions.data}
            columns={revisionColumns}
            rowKey={(revision) => revision.id}
            isLoading={revisions.isLoading}
            isError={revisions.isError}
            error={revisions.error}
            onRetry={() => void revisions.refetch()}
            defaultSort={{ columnId: 'number', direction: 'desc' }}
            emptyState={
              <EmptyState compact title={t('production.selectService')} />
            }
            rowActions={(revision) =>
              revision.active ? (
                <DropdownMenuItem disabled>{t('environments.activeRevision')}</DropdownMenuItem>
              ) : (
                <DropdownMenuItem
                  onSelect={() =>
                    ask({
                      title: t('apps.rollbackTitle', { number: revision.number }),
                      description: t('apps.rollbackWarning'),
                      confirmLabel: t('apps.rollback'),
                      onConfirm: () =>
                        rollback.mutateAsync({ appId: selected.id, revisionId: revision.id }),
                    })
                  }
                >
                  <History aria-hidden />
                  {t('apps.rollback')}
                </DropdownMenuItem>
              )
            }
          />
        </Card>
      ) : null}

      {dialog}
    </div>
  );
}
