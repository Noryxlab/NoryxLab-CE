import * as React from 'react';
import { useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { Cpu, ExternalLink, HardDrive, Layers, Plus, Square, Terminal } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { EmptyState, ErrorState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardFooter } from '@/components/ui/card';
import { StatusBadge, describeStatus } from '@/components/ui/badge';
import { SkeletonCards } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { useEnvironments, useWorkspaces, qk, useInvalidate } from '@/lib/api/queries';
import { workspacesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { WorkspaceStartup } from '@/features/workspaces/workspace-startup';
import { formatDuration, formatQuantity, imageRepository } from '@/lib/format';
import { presentIde } from '@/lib/presenters';
import { LaunchWorkspaceSheet } from '@/features/workspaces/launch-sheet';
import type { Environment, Workspace } from '@/lib/api/types';

function WorkspaceCard({
  workspace,
  environments,
  onStop,
}: {
  workspace: Workspace;
  environments: Environment[];
  onStop: (workspace: Workspace) => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  // The repository is the one thing a pinned image and a tagged environment
  // share, so it is how a card finds what it runs on. And when the environment
  // was rebuilt after this workspace started, the session is running an image
  // the platform no longer serves - Cyril's forty-two-hour Slicer on the old
  // image. "Rebuilt since", not "different revision": a revision carries no
  // digest on this side, and the product must not assert what it cannot show.
  const environment = environments.find(
    (item) => imageRepository(item.destinationImage) === imageRepository(workspace.image),
  );
  const rebuilt = Boolean(
    environment && new Date(environment.updatedAt) > new Date(workspace.createdAt),
  );
  const status = describeStatus(workspace.status, locale);
  const running = status.tone === 'success';

  return (
    <Card className="flex flex-col">
      <CardContent className="flex-1 space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold" title={workspace.name}>
              {workspace.name || presentIde(workspace.kind)}
            </h3>
            <p className="text-xs text-muted-foreground">{presentIde(workspace.kind)}</p>
          </div>
          <StatusBadge status={workspace.status} locale={locale} />
        </div>

        <dl className="grid grid-cols-2 gap-2 text-xs">
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <Cpu className="size-3.5 shrink-0" aria-hidden />
            <dt className="sr-only">{t('workspaces.resources')}</dt>
            <dd className="truncate">
              {formatQuantity(workspace.cpu)} · {formatQuantity(workspace.memory)}
            </dd>
          </div>
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <HardDrive className="size-3.5 shrink-0" aria-hidden />
            <dt className="sr-only">{t('workspaces.storageLabel')}</dt>
            <dd className="truncate">{formatQuantity(workspace.pvcSize)}</dd>
          </div>
        </dl>

        <p className="text-xs text-muted-foreground">
          {t('common.duration')} · {formatDuration(workspace.createdAt, new Date(), locale)}
        </p>
        <p className="flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
          <Layers className="size-3.5 shrink-0" aria-hidden />
          <span className="truncate" title={workspace.image}>
            {environment ? environment.name : t('workspaces.environmentUnknown')}
          </span>
          {rebuilt ? (
            <span
              className="rounded border border-amber-300 bg-amber-50 px-1.5 py-0.5 text-amber-900 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-200"
              title={t('workspaces.rebuiltSince')}
            >
              {t('workspaces.rebuiltSinceShort')}
            </span>
          ) : null}
        </p>
      </CardContent>

      <CardFooter className="justify-between">
        {/* The open action follows the backend status rather than optimistic
            timing — the lesson recorded in docs/onyxia-lessons.md. */}
        {running && workspace.accessUrl ? (
          <Button variant="primary" size="sm" asChild>
            <a href={workspace.accessUrl} target="_blank" rel="noopener noreferrer">
              <ExternalLink aria-hidden />
              {t('common.open')}
            </a>
          </Button>
        ) : (
          <Button variant="primary" size="sm" disabled title={t('workspaces.notReady')}>
            {status.pending ? t('workspaces.openPending') : t('common.open')}
          </Button>
        )}
        <Button variant="ghost" size="sm" onClick={() => onStop(workspace)}>
          <Square aria-hidden />
          {t('workspaces.stop')}
        </Button>
      </CardFooter>
      {/* Pendant l'attente seulement : une fois ouvert, il n'y a plus rien à
          expliquer, et le panneau se retire. */}
      {!running ? <WorkspaceStartup workspaceId={workspace.id} ready={running} /> : null}
    </Card>
  );
}

export function WorkspacesPage() {
  const t = useT();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const [launching, setLaunching] = React.useState(false);

  const { data, isLoading, isError, error, refetch } = useWorkspaces(projectId);
  const environments = useEnvironments(projectId);

  const stop = useMutation({
    mutationFn: (workspaceId: string) => workspacesApi.remove(workspaceId),
    onSuccess: () => {
      invalidate(qk.workspaces(projectId), qk.projects);
      toast.success(t('workspaces.stopped'), t('workspaces.title'));
    },
    onError: (mutationError) => toast.error(mutationError, t('workspaces.stopTitle')),
  });

  function confirmStop(workspace: Workspace) {
    ask({
      title: t('workspaces.stopTitle'),
      description: t('workspaces.stopWarning'),
      confirmLabel: t('workspaces.stop'),
      destructive: true,
      onConfirm: () => stop.mutateAsync(workspace.id),
    });
  }

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('workspaces.title')}
        description={t('workspaces.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setLaunching(true)}>
            <Plus aria-hidden />
            {t('workspaces.create')}
          </Button>
        }
      />

      {isError ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading ? (
        <SkeletonCards count={3} />
      ) : !data || data.length === 0 ? (
        <Card>
          <EmptyState
            icon={Terminal}
            title={t('workspaces.empty')}
            description={t('workspaces.emptyHint')}
            action={
              <Button variant="primary" onClick={() => setLaunching(true)}>
                <Plus aria-hidden />
                {t('workspaces.create')}
              </Button>
            }
          />
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {data.map((workspace) => (
            <WorkspaceCard
              key={workspace.id}
              workspace={workspace}
              environments={environments.data ?? []}
              onStop={confirmStop}
            />
          ))}
        </div>
      )}

      {projectId ? (
        <LaunchWorkspaceSheet projectId={projectId} open={launching} onOpenChange={setLaunching} />
      ) : null}
      {dialog}
      <span className="sr-only" aria-live="polite">
        {data ? `${data.length}` : ''} {locale === 'fr' ? 'workspaces' : 'workspaces'}
      </span>
    </div>
  );
}
