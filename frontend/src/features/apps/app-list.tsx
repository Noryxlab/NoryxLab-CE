import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { AppWindow, ExternalLink, History, Plus, RotateCw, Square, Trash2, Upload } from 'lucide-react';
import { EmptyState } from '@/components/common/states';
import { LogViewer } from '@/components/common/log-viewer';
import { useConfirm } from '@/components/common/confirm-dialog';
import { CopyButton } from '@/components/common/copy-button';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardFooter, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge, StatusBadge, describeStatus } from '@/components/ui/badge';
import { SkeletonCards } from '@/components/ui/skeleton';
import { ErrorState } from '@/components/common/states';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { DataTable, type Column } from '@/components/common/data-table';
import { useToast } from '@/components/ui/toast';
import { appsApi, dashboardsApi } from '@/lib/api/endpoints';
import { useAppLogs, useAppRevisions, qk, useInvalidate } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime, formatRelative } from '@/lib/format';
import { presentAccessMode } from '@/lib/presenters';
import { MoreHorizontal } from 'lucide-react';
import type { App, AppRevision } from '@/lib/api/types';

interface AppListProps {
  projectId: string;
  variant: 'app' | 'dashboard';
  data: App[] | undefined;
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  onRetry: () => void;
  onCreate: () => void;
}

export function AppList({
  projectId,
  variant,
  data,
  isLoading,
  isError,
  error,
  onRetry,
  onCreate,
}: AppListProps) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const [logsFor, setLogsFor] = React.useState<App | null>(null);
  const [revisionsFor, setRevisionsFor] = React.useState<App | null>(null);

  const logs = useAppLogs(logsFor?.id);
  const revisions = useAppRevisions(revisionsFor?.id);

  const refresh = () =>
    invalidate(qk.apps(projectId), qk.dashboards(projectId), qk.production, qk.projects);

  const restart = useMutation({
    mutationFn: (appId: string) => appsApi.restart(appId),
    onSuccess: refresh,
    onError: (mutationError) => toast.error(mutationError, t('apps.restart')),
  });

  const stop = useMutation({
    mutationFn: (appId: string) => appsApi.stop(appId),
    onSuccess: refresh,
    onError: (mutationError) => toast.error(mutationError, t('apps.stop')),
  });

  const publish = useMutation({
    mutationFn: (appId: string) => appsApi.publish(appId),
    onSuccess: () => {
      refresh();
      toast.success(t('apps.publish'), t('production.title'));
    },
    onError: (mutationError) => toast.error(mutationError, t('apps.publishTitle')),
  });

  const rollback = useMutation({
    mutationFn: (input: { appId: string; revisionId: string }) =>
      appsApi.rollback(input.appId, input.revisionId),
    onSuccess: refresh,
    onError: (mutationError) => toast.error(mutationError, t('apps.rollback')),
  });

  const remove = useMutation({
    mutationFn: (appId: string) =>
      variant === 'dashboard' ? dashboardsApi.remove(appId) : appsApi.remove(appId),
    onSuccess: refresh,
    onError: (mutationError) => toast.error(mutationError, t('apps.deleteTitle')),
  });

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
      id: 'publishedBy',
      header: t('production.publishedBy'),
      cell: (revision) => <span className="text-xs text-muted-foreground">{revision.publishedBy || '—'}</span>,
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

  if (isError) return <ErrorState error={error} onRetry={onRetry} />;
  if (isLoading) return <SkeletonCards count={3} />;

  if (!data || data.length === 0) {
    return (
      <Card>
        <EmptyState
          icon={AppWindow}
          title={variant === 'dashboard' ? t('dashboards.empty') : t('apps.empty')}
          description={variant === 'dashboard' ? t('dashboards.emptyHint') : t('apps.emptyHint')}
          action={
            <Button variant="primary" onClick={onCreate}>
              <Plus aria-hidden />
              {variant === 'dashboard' ? t('dashboards.create') : t('apps.create')}
            </Button>
          }
        />
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {data.map((app) => {
          const status = describeStatus(app.status, locale);
          const running = status.tone === 'success';
          return (
            <Card key={app.id} className="flex flex-col">
              <CardContent className="flex-1 space-y-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <h3 className="truncate text-sm font-semibold" title={app.name}>
                      {app.name || app.slug}
                    </h3>
                    <p className="truncate font-mono text-xs text-muted-foreground">/{app.slug}</p>
                  </div>
                  <StatusBadge status={app.status} locale={locale} />
                </div>

                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge tone="outline">{presentAccessMode(app.accessMode, locale)}</Badge>
                  {app.published ? (
                    <Badge tone="brand">
                      {t('production.published')} #{app.activeRevision ?? 1}
                    </Badge>
                  ) : null}
                  {app.restartCount > 0 ? (
                    <Badge tone="warning">
                      {t('apps.restarts')} {app.restartCount}
                    </Badge>
                  ) : null}
                </div>

                {app.healthMessage ? (
                  <p className="line-clamp-2 text-xs text-warning-foreground">{app.healthMessage}</p>
                ) : null}

                <p className="text-xs text-muted-foreground">
                  {formatRelative(app.startedAt ?? app.createdAt, locale)}
                </p>
              </CardContent>

              <CardFooter className="justify-between">
                {running && app.accessUrl ? (
                  <div className="flex items-center gap-1">
                    <Button variant="primary" size="sm" asChild>
                      <a href={app.accessUrl} target="_blank" rel="noopener noreferrer">
                        <ExternalLink aria-hidden />
                        {t('common.open')}
                      </a>
                    </Button>
                    <CopyButton value={app.accessUrl} />
                  </div>
                ) : (
                  <Button variant="primary" size="sm" disabled>
                    {status.pending ? t('workspaces.openPending') : t('common.open')}
                  </Button>
                )}

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon-sm" aria-label={t('common.actions')}>
                      <MoreHorizontal aria-hidden />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent>
                    <DropdownMenuItem onSelect={() => setLogsFor(app)}>
                      {t('common.viewLogs')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => setRevisionsFor(app)}>
                      <History aria-hidden />
                      {t('apps.revisions')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => restart.mutate(app.id)}>
                      <RotateCw aria-hidden />
                      {t('apps.restart')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => stop.mutate(app.id)}>
                      <Square aria-hidden />
                      {t('apps.stop')}
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onSelect={() =>
                        ask({
                          title: t('apps.publishTitle'),
                          description: t('apps.publishHint'),
                          confirmLabel: t('apps.publish'),
                          onConfirm: () => publish.mutateAsync(app.id),
                        })
                      }
                    >
                      <Upload aria-hidden />
                      {t('apps.publish')}
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      destructive
                      onSelect={() =>
                        ask({
                          title: t('apps.deleteTitle'),
                          description: t('apps.deleteWarning'),
                          confirmLabel: t('common.delete'),
                          destructive: true,
                          confirmationValue: app.slug,
                          confirmationLabel: t('apps.slugLabel'),
                          onConfirm: () => remove.mutateAsync(app.id),
                        })
                      }
                    >
                      <Trash2 aria-hidden />
                      {t('common.delete')}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </CardFooter>
            </Card>
          );
        })}
      </div>

      {logsFor ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>
                {t('common.logs')} — {logsFor.name || logsFor.slug}
              </CardTitle>
            </CardHeaderText>
            <Button variant="ghost" size="sm" onClick={() => setLogsFor(null)}>
              {t('common.close')}
            </Button>
          </CardHeader>
          <CardContent>
            <LogViewer
              content={logs.data}
              isLoading={logs.isLoading}
              downloadName={`${logsFor.slug}.log`}
            />
          </CardContent>
        </Card>
      ) : null}

      {revisionsFor ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>
                {t('apps.revisions')} — {revisionsFor.name || revisionsFor.slug}
              </CardTitle>
            </CardHeaderText>
            <Button variant="ghost" size="sm" onClick={() => setRevisionsFor(null)}>
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
              <EmptyState compact title={t('apps.revisions')} description={t('apps.revisionsHint')} />
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
                        rollback.mutateAsync({ appId: revisionsFor.id, revisionId: revision.id }),
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
