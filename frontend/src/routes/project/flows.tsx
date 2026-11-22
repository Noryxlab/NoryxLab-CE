import * as React from 'react';
import { useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { GitFork, Play, Plus, RotateCcw } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { StatusBadge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import { flowsApi } from '@/lib/api/endpoints';
import { qk, useCan, useFlowRun, useFlowRuns, useFlows, useInvalidate } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime, formatDuration } from '@/lib/format';
import { FlowEditorSheet } from '@/features/flows/flow-editor';
import { FlowRunView } from '@/features/flows/flow-run-view';
import { flowColumns } from '@/features/flows/flow-layout';
import type { Flow, FlowRun } from '@/lib/api/types';

/**
 * Les flows d'un projet.
 *
 * Trois niveaux, de haut en bas : les graphes declares, les executions de
 * celui qu'on a choisi, et le detail de celle qu'on a choisie. C'est l'ordre
 * dans lequel la question se pose - « quel pipeline », puis « quelle fois »,
 * puis « qu'est-ce qui s'est passe » - et chaque niveau ne s'ouvre que quand
 * le precedent a une reponse.
 */
export function FlowsPage() {
  const t = useT();
  const can = useCan();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const [editing, setEditing] = React.useState<Flow | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [selectedFlowId, setSelectedFlowId] = React.useState<string | null>(null);
  const [selectedRunId, setSelectedRunId] = React.useState<string | null>(null);

  const flows = useFlows(projectId);
  const selectedFlow = flows.data?.find((flow) => flow.id === selectedFlowId) ?? null;
  const runs = useFlowRuns(selectedFlowId ?? undefined);
  const run = useFlowRun(selectedRunId ?? undefined);

  const start = useMutation({
    mutationFn: ({ flowId, resumeOf }: { flowId: string; resumeOf?: string }) =>
      flowsApi.start(flowId, resumeOf),
    onSuccess: (started) => {
      invalidate(qk.flowRuns(started.flowId));
      setSelectedFlowId(started.flowId);
      // On ouvre l'execution qu'on vient de lancer : c'est la seule chose
      // qu'on veut regarder apres avoir clique.
      setSelectedRunId(started.id);
      toast.success(t('flows.runStarted'), t('flows.title'));
    },
    onError: (error) => toast.error(error, t('flows.run')),
  });

  const flowColumnsList: Column<Flow>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (flow) => flow.name,
      searchValue: (flow) => `${flow.name} ${flow.description ?? ''}`,
      cell: (flow) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{flow.name}</p>
          {flow.description ? (
            <p className="truncate text-xs text-muted-foreground">{flow.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'steps',
      header: t('flows.steps'),
      sortValue: (flow) => flow.steps.length,
      // La forme du graphe en un coup d'oeil : « 4 etapes » ne distingue pas
      // une chaine d'un eventail, et c'est la difference qui compte.
      cell: (flow) => (
        <div className="flex flex-wrap items-center gap-1 text-xs">
          <span className="tabular-nums">{t('flows.stepsCount', { count: String(flow.steps.length) })}</span>
          <span className="text-muted-foreground">
            {flowColumns(flow.steps)
              .map((column) => column.length)
              .join(' → ')}
          </span>
        </div>
      ),
    },
    {
      id: 'maxParallel',
      header: t('flows.maxParallel', { count: '' }).trim(),
      sortValue: (flow) => flow.maxParallel,
      cell: (flow) => <span className="tabular-nums text-muted-foreground">{flow.maxParallel}</span>,
    },
    {
      id: 'createdAt',
      header: t('common.createdAt'),
      sortValue: (flow) => flow.createdAt,
      cell: (flow) => (
        <span className="text-xs text-muted-foreground">{formatDateTime(flow.createdAt, locale)}</span>
      ),
    },
  ];

  const runColumnsList: Column<FlowRun>[] = [
    {
      id: 'startedAt',
      header: t('common.startedAt'),
      sortValue: (item) => item.startedAt,
      cell: (item) => (
        <div className="min-w-0">
          <p className="text-sm">{formatDateTime(item.startedAt, locale)}</p>
          {item.resumeOf ? (
            <p className="text-xs text-muted-foreground">{t('flows.resumeOf')}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (item) => item.status,
      cell: (item) => <StatusBadge status={item.status} locale={locale} showRaw={false} />,
    },
    {
      id: 'duration',
      header: t('common.duration'),
      cell: (item) => (
        <span className="text-xs tabular-nums text-muted-foreground">
          {formatDuration(item.startedAt, item.finishedAt ?? new Date(), locale)}
        </span>
      ),
    },
    {
      id: 'startedBy',
      header: t('common.owner'),
      sortValue: (item) => item.startedBy,
      cell: (item) => <span className="text-xs text-muted-foreground">{item.startedBy || '—'}</span>,
    },
  ];

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('flows.title')}
        description={t('flows.subtitle')}
        actions={
          can('launch-job') ? (
            <Button variant="primary" onClick={() => setCreating(true)}>
              <Plus aria-hidden />
              {t('flows.create')}
            </Button>
          ) : null
        }
      />

      <Card>
        <DataTable
          data={flows.data}
          columns={flowColumnsList}
          rowKey={(flow) => flow.id}
          isLoading={flows.isLoading}
          isError={flows.isError}
          error={flows.error}
          onRetry={() => void flows.refetch()}
          onRowClick={(flow) => {
            setSelectedFlowId(flow.id);
            setSelectedRunId(null);
          }}
          selectedKey={selectedFlowId}
          defaultSort={{ columnId: 'createdAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={GitFork}
              title={t('flows.empty')}
              description={t('flows.emptyHint')}
              action={
                can('launch-job') ? (
                  <Button variant="primary" onClick={() => setCreating(true)}>
                    <Plus aria-hidden />
                    {t('flows.create')}
                  </Button>
                ) : null
              }
            />
          }
          rowActions={(flow) => (
            <>
              {can('launch-job') ? (
                <DropdownMenuItem onSelect={() => start.mutate({ flowId: flow.id })}>
                  <Play aria-hidden />
                  {t('flows.run')}
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuItem
                onSelect={() => {
                  setSelectedFlowId(flow.id);
                  setSelectedRunId(null);
                }}
              >
                {t('flows.runs')}
              </DropdownMenuItem>
              {can('launch-job') ? (
                <DropdownMenuItem onSelect={() => setEditing(flow)}>{t('common.edit')}</DropdownMenuItem>
              ) : null}
            </>
          )}
        />
      </Card>

      {selectedFlow ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{selectedFlow.name}</CardTitle>
              <CardDescription>{t('flows.runs')}</CardDescription>
            </CardHeaderText>
            {can('launch-job') ? (
              <Button variant="secondary" onClick={() => start.mutate({ flowId: selectedFlow.id })}>
                <Play aria-hidden />
                {t('flows.run')}
              </Button>
            ) : null}
          </CardHeader>
          <DataTable
            data={runs.data}
            columns={runColumnsList}
            rowKey={(item) => item.id}
            isLoading={runs.isLoading}
            isError={runs.isError}
            error={runs.error}
            onRetry={() => void runs.refetch()}
            onRowClick={(item) => setSelectedRunId(item.id)}
            selectedKey={selectedRunId}
            defaultSort={{ columnId: 'startedAt', direction: 'desc' }}
            emptyState={
              <EmptyState
                compact
                icon={Play}
                title={t('flows.runsEmpty')}
                description={t('flows.runsEmptyHint')}
              />
            }
            rowActions={(item) => (
              <>
                {/* Reprendre n'est propose que sur une execution terminee et
                  *  non reussie : reprendre une execution reussie ne
                  *  reutiliserait que des etapes deja faites, et reprendre
                  *  celle qui tourne encore lancerait deux fois les memes
                  *  jobs. */}
                {can('launch-job') && (item.status === 'failed' || item.status === 'cancelled') ? (
                  <DropdownMenuItem
                    onSelect={() =>
                      ask({
                        title: t('flows.resumeTitle'),
                        description: t('flows.resumeHint'),
                        confirmLabel: t('flows.resume'),
                        onConfirm: () =>
                          start.mutateAsync({ flowId: item.flowId, resumeOf: item.id }),
                      })
                    }
                  >
                    <RotateCcw aria-hidden />
                    {t('flows.resume')}
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuItem onSelect={() => setSelectedRunId(item.id)}>
                  {t('common.open')}
                </DropdownMenuItem>
              </>
            )}
          />
        </Card>
      ) : null}

      {selectedFlow && run.data ? (
        <FlowRunView flow={selectedFlow} run={run.data.run} steps={run.data.steps ?? []} />
      ) : null}

      {projectId ? (
        <>
          <FlowEditorSheet
            projectId={projectId}
            flow={null}
            open={creating}
            onOpenChange={setCreating}
          />
          <FlowEditorSheet
            projectId={projectId}
            flow={editing}
            open={editing !== null}
            onOpenChange={(open) => !open && setEditing(null)}
          />
        </>
      ) : null}
      {dialog}
    </div>
  );
}
